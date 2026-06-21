package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelsapi"
)

func TestListAnthropic_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method: %s", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("x-api-key: got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("anthropic-version: got %q", got)
		}
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "claude-sonnet-4-6", "type": "model"},
				{"id": "claude-opus-4-8", "type": "model"},
			},
			"has_more": false,
		}
		if r.URL.Query().Get("after_id") != "" {
			resp["data"] = []map[string]any{{"id": "claude-haiku-4-5-20251001", "type": "model"}}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ids, err := modelsapi.ListAnthropic(srv.URL, "test-key", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("ids: %v", ids)
	}
	if ids[0] != "claude-sonnet-4-6" {
		t.Fatalf("first model: %q", ids[0])
	}
}

func TestListAnthropic_OAuthBearer_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oat-test" {
			t.Fatalf("Authorization: got %q", got)
		}
		if got := r.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Fatalf("anthropic-beta: got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":     []map[string]any{{"id": "claude-sonnet-4-6", "type": "model"}},
			"has_more": false,
		})
	}))
	defer srv.Close()

	ids, err := modelsapi.ListAnthropic(srv.URL, "oat-test", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "claude-sonnet-4-6" {
		t.Fatalf("ids: %v", ids)
	}
}

func TestPickAnthropicFlagshipModels(t *testing.T) {
	ids := []string{
		"claude-sonnet-4-20250514",
		"claude-sonnet-4-5-20250929",
		"claude-sonnet-4-6",
		"claude-opus-4-6",
		"claude-opus-4-8",
		"claude-opus-4-20250514",
		"claude-haiku-3-5-20241022",
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-6-thinking",
	}
	got := modelsapi.PickAnthropicFlagshipModels(ids)
	want := []string{
		"claude-opus-4-8",
		"claude-sonnet-4-6",
		"claude-haiku-4-5-20251001",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %q want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestPickAnthropicFlagshipModels_PrefersHigherMinor(t *testing.T) {
	ids := []string{"claude-sonnet-4-5", "claude-sonnet-4-5-20250929", "claude-sonnet-4-6"}
	got := modelsapi.PickAnthropicFlagshipModels(ids)
	if len(got) != 1 || got[0] != "claude-sonnet-4-6" {
		t.Fatalf("got %v", got)
	}
}

func TestCuratedAnthropicModels_NoRetiredIDs(t *testing.T) {
	for _, id := range modelsapi.CuratedAnthropicModels() {
		if strings.Contains(id, "20250514") {
			t.Fatalf("retired model in curated list: %q", id)
		}
	}
}
