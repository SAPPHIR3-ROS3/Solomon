package test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

type subscriptionTestTransport func(*http.Request) (*http.Response, error)

func (f subscriptionTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSubscriptionConnectKeepsCredentialsWhenCatalogFails(t *testing.T) {
	for _, name := range []string{config.ProviderNameChatGPTSub, config.ProviderNameClaudeSub} {
		t.Run(name, func(t *testing.T) {
			if name == config.ProviderNameChatGPTSub {
				requireCodexCallbackPort(t)
			}
			t.Setenv("SOLOMON_HOME", t.TempDir())
			root := config.EmptyRoot()
			config.AppendOrUpdateProvider(root, config.Provider{Name: "Existing", BaseURL: "https://example.com/v1/", APIKey: "existing-key"})
			root.Current = config.Current{Provider: "Existing", Model: "existing-model"}
			if err := config.Save(root); err != nil {
				t.Fatal(err)
			}
			// A browser login can take minutes: changes during it must survive the save.
			if _, err := config.UpdateReasoningEffort("low"); err != nil {
				t.Fatal(err)
			}
			sawSavedCredentials := false
			modelFailure := func(w http.ResponseWriter, r *http.Request) {
				saved, err := config.Load()
				if err != nil {
					t.Error(err)
				} else if p := config.ProviderByName(saved, name); p != nil && p.OAuthRefreshToken == "new-refresh" {
					sawSavedCredentials = true
				}
				w.WriteHeader(http.StatusServiceUnavailable)
				io.WriteString(w, `{"error":{"type":"api_error","message":"catalog unavailable"}}`)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`)
				case "/latest":
					io.WriteString(w, `{"version":"0.153.3"}`)
				default:
					modelFailure(w, r)
				}
			}))
			defer server.Close()
			defer claude.SetTokenEndpointForTest(server.URL + "/token")()
			defer codex.SetTokenEndpointForTest(server.URL + "/token")()
			base, registry := codex.ChatGPTSubAPIBase, codex.CodexVersionRegistryURL
			codex.ChatGPTSubAPIBase = server.URL
			codex.CodexVersionRegistryURL = server.URL + "/latest"
			defer func() { codex.ChatGPTSubAPIBase = base; codex.CodexVersionRegistryURL = registry }()
			transport := http.DefaultTransport
			http.DefaultTransport = subscriptionTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "api.anthropic.com" {
					recorder := httptest.NewRecorder()
					modelFailure(recorder, r)
					return recorder.Result(), nil
				}
				return transport.RoundTrip(r)
			})
			defer func() { http.DefaultTransport = transport }()
			kind := config.ProviderKindChatGPTSub
			if name == config.ProviderNameClaudeSub {
				kind = config.ProviderKindClaudeSub
			}
			_, err := config.RunProviderSetupByKind(t.Context(), config.PromptIO{Out: io.Discard}, root, root, kind, config.ProviderSetupOpts{
				WriteToConfig: true,
				SaveProvider: func(p config.Provider) error {
					latest, err := config.UpdateProvider(p)
					if err == nil {
						*root = *latest
					}
					return err
				},
				OpenBrowser: func(raw string) error {
					u, err := url.Parse(raw)
					if err != nil {
						return err
					}
					q := u.Query()
					response, err := http.Get(q.Get("redirect_uri") + "?code=single-use-code&state=" + url.QueryEscape(q.Get("state")))
					if err != nil {
						return err
					}
					defer response.Body.Close()
					io.Copy(io.Discard, response.Body)
					if response.StatusCode != http.StatusOK {
						t.Errorf("callback status=%d", response.StatusCode)
					}
					return nil
				},
			})
			if err == nil || !strings.Contains(err.Error(), "credentials saved") {
				t.Fatalf("error=%v", err)
			}
			if !sawSavedCredentials {
				t.Fatal("model discovery ran before credentials were persisted")
			}
			saved, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			p := config.ProviderByName(saved, name)
			if p == nil || p.OAuthRefreshToken != "new-refresh" {
				t.Fatal("catalog failure discarded login")
			}
			if saved.ReasoningEffort != "low" || saved.Current.Provider != "Existing" || config.ProviderByName(saved, "Existing") == nil {
				t.Fatal("login overwrote newer settings or existing connection")
			}
		})
	}
}
