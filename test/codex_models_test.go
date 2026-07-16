package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex"
)

func TestCodexListModels_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method: %s", r.Method)
		}
		if r.URL.Path != "/backend-api/codex/models" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization: %q", got)
		}
		if got := r.Header.Get("chatgpt-account-id"); got != "acct-1" {
			t.Fatalf("chatgpt-account-id: %q", got)
		}
		if got := r.URL.Query().Get("client_version"); got != codex.ClientVersion {
			t.Fatalf("client_version: %q want %q", got, codex.ClientVersion)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"slug": "gpt-5.6-sol", "visibility": "list", "priority": 20},
				{"slug": "gpt-5.6-terra", "visibility": "list", "priority": 19},
				{"slug": "gpt-5.6-luna", "visibility": "list", "priority": 18},
				{"slug": "gpt-hidden", "visibility": "hidden", "priority": 99},
				{"slug": "gpt-5.5", "visibility": "list", "priority": 10},
				{"slug": "gpt-5.4-mini", "visibility": "list", "priority": 7},
			},
		})
	}))
	defer srv.Close()

	prev := codex.ChatGPTSubAPIBase
	codex.ChatGPTSubAPIBase = srv.URL + "/backend-api/codex"
	defer func() { codex.ChatGPTSubAPIBase = prev }()

	ids, err := codex.ListModels(context.Background(), "test-token", "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini"}
	if len(ids) != len(want) {
		t.Fatalf("ids: %v want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("index %d: got %q want %q (full %v)", i, ids[i], want[i], ids)
		}
	}
}
