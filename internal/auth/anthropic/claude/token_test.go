package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefreshOmitsScope(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"refresh_token": "rt2",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	old := tokenEndpoint
	tokenEndpoint = srv.URL
	defer func() { tokenEndpoint = old }()

	_, err := Refresh(t.Context(), "rt")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotBody["scope"]; ok {
		t.Fatalf("refresh must not send scope, got %#v", gotBody)
	}
	if gotBody["grant_type"] != "refresh_token" {
		t.Fatalf("grant_type = %q", gotBody["grant_type"])
	}
}
