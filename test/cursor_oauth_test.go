package test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestCursorAuthParamsLoginURL(t *testing.T) {
	params, err := cursorauth.GenerateAuthParams()
	if err != nil {
		t.Fatal(err)
	}
	if params.Verifier == "" || params.Challenge == "" || params.UUID == "" {
		t.Fatalf("incomplete params: %+v", params)
	}
	for _, want := range []string{
		"https://cursor.com/loginDeepControl?",
		"challenge=" + params.Challenge,
		"uuid=" + params.UUID,
		"mode=login",
		"redirectTarget=cli",
	} {
		if !strings.Contains(params.LoginURL, want) {
			t.Fatalf("login URL missing %q: %s", want, params.LoginURL)
		}
	}
}

func TestCursorPollAndRefresh(t *testing.T) {
	var polls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		if n == 1 {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access-1",
			"refreshToken": "refresh-1",
		})
	})
	mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer refresh-1" {
			t.Errorf("refresh auth=%q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": "access-2"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	restore := cursorauth.SetEndpointsForTest(srv.URL+"/poll", srv.URL+"/refresh", srv.URL+"/mint")
	defer restore()

	tokens, err := cursorauth.PollForAuth(context.Background(), "id", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "access-1" || tokens.RefreshToken != "refresh-1" {
		t.Fatalf("poll tokens=%+v", tokens)
	}

	refreshed, err := cursorauth.Refresh(context.Background(), "refresh-1")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken != "access-2" || refreshed.RefreshToken != "refresh-1" {
		t.Fatalf("refresh tokens=%+v", refreshed)
	}
}

func TestCursorMintUserAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-1" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"apiKey": "crsr_from_login"})
	}))
	defer srv.Close()
	restore := cursorauth.SetEndpointsForTest(srv.URL+"/poll", srv.URL+"/refresh", srv.URL)
	defer restore()
	key, err := cursorauth.MintUserAPIKey(context.Background(), "access-1")
	if err != nil {
		t.Fatal(err)
	}
	if key != "crsr_from_login" {
		t.Fatalf("key=%q", key)
	}
}

func TestCursorListAvailableModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-1" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []any{
				map[string]any{"name": "gpt-5", "variants": []map[string]string{{"name": "gpt-5-fast"}, {"id": "fast"}}},
				map[string]any{"name": "gpt-5-fast"},
				map[string]any{"name": "claude-opus-4-5"},
				map[string]any{"name": "composer-2.5"},
			},
		})
	}))
	defer srv.Close()
	old := cursorauth.SetModelsEndpointForTest(srv.URL)
	defer old()
	ids, err := cursorauth.ListAvailableModels(context.Background(), "access-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 4 || ids[0] != "gpt-5" {
		t.Fatalf("ids=%v", ids)
	}
}

func TestCursorKeepsCursorModelPrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []any{
				map[string]any{"name": "cursor-grok-4.5"},
				map[string]any{"name": "composer-2.5"},
			},
		})
	}))
	defer srv.Close()
	old := cursorauth.SetModelsEndpointForTest(srv.URL)
	defer old()
	ids, err := cursorauth.ListAvailableModels(context.Background(), "access-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "cursor-grok-4.5,composer-2.5" {
		t.Fatalf("ids=%v", ids)
	}
}

func TestCursorSkipsUnavailableModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []any{
				map[string]any{"name": "grok-4.5", "available": false},
				map[string]any{"name": "composer-2.5"},
				map[string]any{"name": "gpt-5", "status": "deprecated"},
			},
		})
	}))
	defer srv.Close()
	old := cursorauth.SetModelsEndpointForTest(srv.URL)
	defer old()
	ids, err := cursorauth.ListAvailableModels(context.Background(), "access-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "composer-2.5" {
		t.Fatalf("ids=%v", ids)
	}
}

func TestCursorOAuthProviderCredentials(t *testing.T) {
	p := config.Provider{
		Name:     config.ProviderNameCursorSub,
		BaseURL:  cursorauth.APIBase,
		AuthKind: config.AuthKindOAuthCursor,
	}
	config.ApplyCursorOAuthTokens(&p, cursorauth.TokenSet{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if p.IsCursorAPI() || !p.IsCursorSub() || !p.IsOAuthProvider() || !config.ProviderCredentialsReady(&p) {
		t.Fatalf("provider not ready: %+v", p)
	}
	cfg := &config.Root{}
	config.AppendOrUpdateProvider(cfg, p)
	if config.CursorAPIConfigured(cfg) {
		t.Fatal("Cursor Sub must not count as Cursor API")
	}
	got, err := config.ResolveProviderBearer(context.Background(), nil, &p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "access" {
		t.Fatalf("bearer=%q", got)
	}
	p.APIKey = "crsr_minted"
	got, err = config.ResolveProviderBearer(context.Background(), nil, &p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "crsr_minted" {
		t.Fatalf("api key bearer=%q", got)
	}
	session, err := config.ResolveCursorSessionBearer(context.Background(), nil, &p)
	if err != nil {
		t.Fatal(err)
	}
	if session != "access" {
		t.Fatalf("session=%q", session)
	}
	sessionOnly := config.Provider{
		Name:     config.ProviderNameCursorSub,
		BaseURL:  cursorauth.APIBase,
		AuthKind: config.AuthKindOAuthCursor,
		APIKey:   "session-jwt.payload.sig",
	}
	got, err = config.ResolveCursorSessionBearer(context.Background(), nil, &sessionOnly)
	if err != nil {
		t.Fatal(err)
	}
	if got != "session-jwt.payload.sig" {
		t.Fatalf("session from api key field=%q", got)
	}
}

func TestDedupeCursorVariantIDs(t *testing.T) {
	got := cursorauth.DedupeVariantIDs([]string{
		"gpt-5-fast",
		"gpt-5-high",
		"gpt-5-low",
		"gpt-5-xhigh",
		"gpt-5-thinking",
		"gpt-5-none",
		"gpt-5",
		"none",
		"composer-2.5-fast",
		"fast",
		"claude-opus-4-5",
	})
	want := []string{"gpt-5", "composer-2.5", "claude-opus-4-5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
	caps := cursorauth.DedupeVariantFlags([]cursorauth.ModelFlag{
		{ID: "fable"},
		{ID: "gpt-5-fast", Fast: true},
		{ID: "gpt-5", Thinking: true},
		{ID: "gpt-5-high"},
		{ID: "gpt-5-low"},
		{ID: "composer-2.5-thinking", Thinking: true},
	})
	if strings.Join(caps.Fast, ",") != "gpt-5" {
		t.Fatalf("fast=%v", caps.Fast)
	}
	if strings.Join(caps.ThinkingToggle, ",") != "composer-2.5" {
		t.Fatalf("toggle=%v", caps.ThinkingToggle)
	}
	if strings.Join(caps.ThinkingLevels, ",") != "gpt-5" {
		t.Fatalf("levels=%v", caps.ThinkingLevels)
	}
}

func TestResolveChatModelID(t *testing.T) {
	caps := cursorauth.DedupeVariantFlags([]cursorauth.ModelFlag{
		{ID: "grok-4.6"},
		{ID: "grok-4.6-fast"},
		{ID: "grok-4.6-thinking-medium"},
		{ID: "grok-4.6-thinking-medium-fast"},
	})
	cursorauth.SetModelCaps(caps)
	t.Cleanup(func() { cursorauth.SetModelCaps(cursorauth.ModelCaps{}) })
	got := cursorauth.ResolveChatModelID("grok-4.6", "medium", true)
	if got != "cursor-grok-4.6-medium-fast" {
		t.Fatalf("got %q", got)
	}
}

func TestCursorStreamChat(t *testing.T) {
	mockCursorInferenceCatalog(t)
	payload := []byte{0x0a, 0x07, 0x0a, 0x05, 'h', 'e', 'l', 'l', 'o'}
	frame := make([]byte, 5+len(payload))
	frame[1] = 0
	frame[2] = 0
	frame[3] = 0
	frame[4] = byte(len(payload))
	copy(frame[5:], payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer session" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("x-cursor-client-version") == "" {
			t.Errorf("missing client version header")
		}
		sum := r.Header.Get("x-cursor-checksum")
		slash := strings.IndexByte(sum, '/')
		if slash < 9 || slash > 80 {
			t.Errorf("checksum=%q", sum)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "cursor-grok-4.5") && !strings.Contains(string(raw), "grok-4.5") {
			t.Errorf("model missing in body")
		}
		_, _ = w.Write(frame)
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	}))
	defer srv.Close()
	restore := cursorauth.SetChatEndpointForTest(srv.URL)
	defer restore()
	var buf strings.Builder
	got, err := cursorauth.StreamChat(context.Background(), "session", "cursor-grok-4.5", []cursorauth.ChatTurn{{Role: "user", Content: "hi"}}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" || buf.String() != "hello" {
		t.Fatalf("got=%q buf=%q", got, buf.String())
	}
}

func TestCursorStreamChatTextFrame(t *testing.T) {
	mockCursorInferenceCatalog(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "grok-4.6") {
			t.Errorf("model missing in body")
		}
		_, _ = w.Write(cursorInferenceTestFrame([]byte{10, 6, 10, 4, 'c', 'i', 'a', 'o'}, 0))
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	}))
	defer srv.Close()
	restore := cursorauth.SetChatEndpointForTest(srv.URL)
	defer restore()
	got, err := cursorauth.StreamChat(context.Background(), "session", "grok-4.6", []cursorauth.ChatTurn{{Role: "user", Content: "hi"}}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ciao" {
		t.Fatalf("got=%q", got)
	}
}

func TestCursorStreamChatIgnoresMetadata(t *testing.T) {
	mockCursorInferenceCatalog(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(cursorInferenceTestFrame([]byte{10, 4, 10, 2, 'O', 'K'}, 0))
		_, _ = w.Write(cursorInferenceTestFrame([]byte{34, 7, 10, 5, 'E', 'r', 'r', 'o', 'r'}, 0))
		_, _ = w.Write(cursorInferenceTestFrame([]byte(`{}`), 2))
	}))
	defer srv.Close()
	restore := cursorauth.SetChatEndpointForTest(srv.URL)
	defer restore()
	got, err := cursorauth.StreamChat(context.Background(), "session", "grok-4.6", []cursorauth.ChatTurn{{Role: "user", Content: "hi"}}, io.Discard)
	if err != nil || got != "OK" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestCursorStreamChatRejectsRPCErrorText(t *testing.T) {
	mockCursorInferenceCatalog(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "Invalid request because first message is not a streamUnifiedChatRequest"},
		})
	}))
	defer srv.Close()
	restore := cursorauth.SetChatEndpointForTest(srv.URL)
	defer restore()
	_, err := cursorauth.StreamChat(context.Background(), "session", "grok-4.6", []cursorauth.ChatTurn{{Role: "user", Content: "hi"}}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "streamUnifiedChatRequest") {
		t.Fatalf("err=%v", err)
	}
}

func cursorInferenceTestFrame(payload []byte, flag byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = flag
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

func mockCursorInferenceCatalog(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request["useModelParameters"] != true {
			t.Errorf("parameterized catalog not requested")
		}
		_, _ = io.WriteString(w, `{"models":[{"name":"grok-4.5","variants":[{"legacySlug":"cursor-grok-4.5","parameterValues":[]}]},{"name":"grok-4.6"}]}`)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(cursorauth.SetModelsEndpointForTest(srv.URL))
}
