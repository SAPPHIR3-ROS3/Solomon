package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/tooloutput"
)

func TestCleanupProjectTempRemovesDirectory(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	dir, err := chatstore.TempDir(testProjectHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spill.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tooloutput.CleanupProjectTemp(testProjectHex); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temp dir should be removed, stat err=%v", err)
	}
}

func TestRuntimeCloseCleansToolTempWhenLastInstance(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	t.Cleanup(func() { _ = tooloutput.CleanupProjectTemp(testProjectHex) })
	dir, err := chatstore.TempDir(testProjectHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &config.Provider{Name: "p", BaseURL: "http://127.0.0.1:9", APIKey: "k"}
	cfg := &config.Root{
		Current:   config.Current{Provider: "p", Model: "m"},
		Providers: map[string]*config.Provider{"p": p},
	}
	rt := agentruntime.NewRuntime(nil, cfg, p, testProjectHex, t.TempDir(), &chatstore.Session{ID: "cleanup-test"})
	rt.ToolOut.MarkSpillGenerated()
	if err := rt.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Runtime.Close should remove temp dir when last instance, stat err=%v", err)
	}
}

func TestCloseProjectTempDefersWhenOthersActive(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	t.Cleanup(func() {
		_ = tooloutput.CleanupProjectTemp(testProjectHex)
		home, _ := paths.SolomonHome()
		_ = os.Remove(filepath.Join(home, "temp.txt"))
	})
	dir, err := chatstore.TempDir(testProjectHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spill.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tooloutput.CloseProjectTemp(testProjectHex, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("temp dir should remain when others active: %v", err)
	}
	home, err := paths.SolomonHome()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, "temp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), testProjectHex) {
		t.Fatalf("temp.txt missing project hex: %q", string(b))
	}
}

func TestCloseProjectTempFlushesDeferredOnLastInstance(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	otherHex := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	t.Cleanup(func() {
		_ = tooloutput.CleanupProjectTemp(testProjectHex)
		_ = tooloutput.CleanupProjectTemp(otherHex)
		home, _ := paths.SolomonHome()
		_ = os.Remove(filepath.Join(home, "temp.txt"))
	})
	home, err := paths.SolomonHome()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "temp.txt"), []byte(otherHex+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	otherDir, err := chatstore.TempDir(otherHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "x.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tooloutput.CloseProjectTemp(testProjectHex, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(otherDir); !os.IsNotExist(err) {
		t.Fatalf("deferred temp dir should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "temp.txt")); !os.IsNotExist(err) {
		t.Fatal("temp.txt should be cleared after flush")
	}
}
