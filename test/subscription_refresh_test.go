package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func subscriptionRefreshFixture(t *testing.T, name string) (*config.Root, *atomic.Int32) {
	t.Helper()
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	kind := config.AuthKindOAuthChatGPT
	if name == config.ProviderNameClaudeSub {
		kind = config.AuthKindOAuthClaude
	}
	p := config.Provider{Name: name, BaseURL: "https://example.com/v1/", AuthKind: kind, OAuthAccessToken: "old-access", OAuthRefreshToken: "single-use-refresh", OAuthExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339), OAuthAccountID: "saved-account"}
	config.AppendOrUpdateProvider(root, p)
	root.Current = config.Current{Provider: name, Model: "audit-model"}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"version":"0.153.3"}`)
			return
		}
		requests.Add(1)
		var refresh string
		if name == config.ProviderNameClaudeSub {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			refresh = body["refresh_token"]
		} else {
			r.ParseForm()
			refresh = r.Form.Get("refresh_token")
		}
		if refresh != "single-use-refresh" || requests.Load() > 1 {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"error":"invalid_grant"}`)
			return
		}
		io.WriteString(w, `{"access_token":"fresh-access","refresh_token":"rotated-refresh","expires_in":3600}`)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(claude.SetTokenEndpointForTest(server.URL))
	t.Cleanup(codex.SetTokenEndpointForTest(server.URL))
	registry := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = server.URL
	t.Cleanup(func() { codex.CodexVersionRegistryURL = registry })
	return root, requests
}

func TestSubscriptionRefreshPersistsProviderCopy(t *testing.T) {
	for _, name := range []string{config.ProviderNameChatGPTSub, config.ProviderNameClaudeSub} {
		t.Run(name, func(t *testing.T) {
			root, requests := subscriptionRefreshFixture(t, name)
			copy := config.ProviderList(root)[0]
			token, err := config.ResolveProviderBearer(t.Context(), root, &copy)
			if err != nil || token != "fresh-access" {
				t.Fatalf("token=%q err=%v", token, err)
			}
			saved, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 1 || config.ProviderByName(saved, name).OAuthRefreshToken != "rotated-refresh" || config.ProviderByName(root, name).OAuthRefreshToken != "rotated-refresh" {
				t.Fatal("rotation not persisted to config and caller")
			}
			if name == config.ProviderNameChatGPTSub && copy.OAuthAccountID != "saved-account" {
				t.Fatal("refresh lost account ID")
			}
		})
	}
}

func TestSubscriptionRefreshCoordinatesSnapshots(t *testing.T) {
	for _, name := range []string{config.ProviderNameChatGPTSub, config.ProviderNameClaudeSub} {
		t.Run(name, func(t *testing.T) {
			_, requests := subscriptionRefreshFixture(t, name)
			var snapshots []*config.Root
			for i := 0; i < 6; i++ {
				root, err := config.Load()
				if err != nil {
					t.Fatal(err)
				}
				snapshots = append(snapshots, root)
			}
			if _, err := config.UpdateReasoningEffort("low"); err != nil {
				t.Fatal(err)
			}
			var group sync.WaitGroup
			for _, root := range snapshots {
				group.Add(1)
				go func(root *config.Root) {
					defer group.Done()
					token, err := config.ResolveProviderBearer(context.Background(), root, config.ProviderByName(root, name))
					if err != nil || token != "fresh-access" {
						t.Errorf("token=%q err=%v", token, err)
					}
				}(root)
			}
			group.Wait()
			saved, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 1 {
				t.Fatalf("grant spent %d times", requests.Load())
			}
			if saved.ReasoningEffort != "low" {
				t.Fatal("refresh overwrote newer setting")
			}
		})
	}
}

func TestSubscriptionRefreshRetriesDiskCommit(t *testing.T) {
	for _, name := range []string{config.ProviderNameChatGPTSub, config.ProviderNameClaudeSub} {
		t.Run(name, func(t *testing.T) {
			root, requests := subscriptionRefreshFixture(t, name)
			path := filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml.tmp")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			p := config.ProviderByName(root, name)
			if _, err := config.ResolveProviderBearer(t.Context(), root, p); err == nil {
				t.Fatal("ignored disk failure")
			}
			if p.OAuthAccessToken != "old-access" {
				t.Fatal("uncommitted token exposed")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			token, err := config.ResolveProviderBearer(t.Context(), root, p)
			if err != nil || token != "fresh-access" || requests.Load() != 1 {
				t.Fatalf("token=%q err=%v requests=%d", token, err, requests.Load())
			}
		})
	}
}

func TestChatGPTRefreshErrorNamesProviderWithoutLeakingBody(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"version":"0.153.3"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"code":"invalid_refresh_token","message":"secret-must-not-escape"}}`)
	}))
	defer server.Close()
	defer codex.SetTokenEndpointForTest(server.URL)()
	registry := codex.CodexVersionRegistryURL
	codex.CodexVersionRegistryURL = server.URL
	defer func() { codex.CodexVersionRegistryURL = registry }()
	_, err := codex.Refresh(t.Context(), "fake-refresh")
	if err == nil || !strings.Contains(err.Error(), "ChatGPT Sub") || !strings.Contains(err.Error(), "invalid_refresh_token") || strings.Contains(err.Error(), "secret-must-not-escape") {
		t.Fatalf("error=%v", err)
	}
}
