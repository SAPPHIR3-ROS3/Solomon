package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/anthropic/claude"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func expiredClaudeConfig(t *testing.T) *config.Root {
	t.Helper()
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	provider, err := config.NewClaudeSubProvider(claude.TokenSet{AccessToken: "old-access", RefreshToken: "single-use-refresh", ExpiresAt: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	root.Providers = map[string]*config.Provider{provider.Name: &provider}
	root.Current = config.Current{Provider: provider.Name, Model: "claude-sonnet-4-6"}
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestClaudeOAuthRefreshCoordinatesStaleSnapshots(t *testing.T) {
	root := expiredClaudeConfig(t)
	snapshots := []*config.Root{root}
	for i := 0; i < 5; i++ {
		snapshot, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, snapshot)
	}
	// A targeted settings update after the runtime snapshots must survive refresh.
	if _, err := config.UpdateReasoningEffort("low"); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["refresh_token"] != "single-use-refresh" {
			t.Errorf("refresh=%q", body["refresh_token"])
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"fresh-access","refresh_token":"rotated-refresh","expires_in":3600}`)
	}))
	defer srv.Close()
	restore := claude.SetTokenEndpointForTest(srv.URL)
	defer restore()
	var group sync.WaitGroup
	for _, snapshot := range snapshots {
		group.Add(1)
		go func(root *config.Root) {
			defer group.Done()
			provider := config.ProviderByName(root, config.ProviderNameClaudeSub)
			token, err := config.ResolveProviderBearer(context.Background(), root, provider)
			if err != nil || token != "fresh-access" {
				t.Errorf("token=%q err=%v", token, err)
			}
		}(snapshot)
	}
	group.Wait()
	if requests.Load() != 1 {
		t.Fatalf("single-use refresh spent %d times", requests.Load())
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.ReasoningEffort != "low" {
		t.Fatal("refresh overwrote a newer setting")
	}
	provider := config.ProviderByName(saved, config.ProviderNameClaudeSub)
	if provider.OAuthRefreshToken != "rotated-refresh" {
		t.Fatal("rotated token not saved")
	}
	home := os.Getenv("SOLOMON_HOME")
	stat, err := os.Stat(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && stat.Mode().Perm()&0o077 != 0 {
		t.Fatalf("credential permissions=%o", stat.Mode().Perm())
	}
}

func TestClaudeOAuthRefreshRetriesPersistenceWithoutReusingGrant(t *testing.T) {
	root := expiredClaudeConfig(t)
	tempPath := filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml.tmp")
	if err := os.Mkdir(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, `{"access_token":"committed-access","refresh_token":"committed-refresh","expires_in":3600}`)
	}))
	defer srv.Close()
	restore := claude.SetTokenEndpointForTest(srv.URL)
	defer restore()
	provider := config.ProviderByName(root, config.ProviderNameClaudeSub)
	if _, err := config.ResolveProviderBearer(context.Background(), root, provider); err == nil {
		t.Fatal("ignored persistence failure")
	}
	if provider.OAuthAccessToken != "old-access" {
		t.Fatal("uncommitted credential exposed to inference")
	}
	if err := os.Remove(tempPath); err != nil {
		t.Fatal(err)
	}
	token, err := config.ResolveProviderBearer(context.Background(), root, provider)
	if err != nil || token != "committed-access" {
		t.Fatalf("token=%q error=%v", token, err)
	}
	if requests.Load() != 1 {
		t.Fatal("single-use refresh replayed after disk failure")
	}
}
