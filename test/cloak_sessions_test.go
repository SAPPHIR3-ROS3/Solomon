package test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

func TestCloakSessionsReuseAndIsolation(t *testing.T) {
	created := 0
	sessions := cloak.NewSessions(context.Background(), func() (*cloak.Client, error) { created++; return &cloak.Client{}, nil })
	defer sessions.Close()
	first, err := sessions.Get("project", "chat")
	if err != nil {
		t.Fatal(err)
	}
	again, err := sessions.Get("project", "chat")
	if err != nil || again != first || created != 1 {
		t.Fatalf("browser not reused: %v", err)
	}
	other, err := sessions.Get("project", "other")
	if err != nil || other == first {
		t.Fatal("chats share a browser")
	}
	differentProject, err := sessions.Get("other", "chat")
	if err != nil || differentProject == first {
		t.Fatal("projects share a browser")
	}
	runtime := &agentruntime.Runtime{Cloak: first, CloakShared: true}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	reused, err := sessions.Get("project", "chat")
	if err != nil || reused != first {
		t.Fatal("runtime close lost session browser")
	}
	if err := sessions.Remove("project", "chat"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.CallTool(t.Context(), "list_pages", nil, "verify removed browser"); !errors.Is(err, cloak.ErrUnavailable) {
		t.Fatalf("removed browser remains available: %v", err)
	}
	replacement, err := sessions.Get("project", "chat")
	if err != nil || replacement == first {
		t.Fatal("removed session was not replaced")
	}
	if err := sessions.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Get("project", "chat"); !errors.Is(err, cloak.ErrUnavailable) {
		t.Fatal("closed registry accepted session")
	}
}

func TestCloakSessionsConcurrentAcquire(t *testing.T) {
	sessions := cloak.NewSessions(context.Background(), func() (*cloak.Client, error) { return &cloak.Client{}, nil })
	defer sessions.Close()
	const count = 20
	clients := make(chan *cloak.Client, count)
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			client, err := sessions.Get("project", "chat")
			if err != nil {
				t.Error(err)
				return
			}
			clients <- client
		}()
	}
	group.Wait()
	close(clients)
	var first *cloak.Client
	for client := range clients {
		if first == nil {
			first = client
		}
		if client != first {
			t.Fatal("concurrent acquisition created multiple browsers")
		}
	}
}

func TestCloakSessionsShutdownAndFactoryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sessions := cloak.NewSessions(ctx, func() (*cloak.Client, error) { return &cloak.Client{}, nil })
	defer sessions.Close()
	client, err := sessions.Get("project", "chat")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		_, err = sessions.Get("project", "chat")
		if errors.Is(err, cloak.ErrUnavailable) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shutdown did not release browsers")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := client.CallTool(t.Context(), "list_pages", nil, "verify shutdown"); !errors.Is(err, cloak.ErrUnavailable) {
		t.Fatal(err)
	}
	failure := errors.New("factory failure")
	failed := cloak.NewSessions(context.Background(), func() (*cloak.Client, error) { return nil, failure })
	defer failed.Close()
	if _, err := failed.Get("project", "chat"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := failed.Get("", ""); err == nil {
		t.Fatal("empty identity accepted")
	}
}

func TestCloakRuntimeClosesOwnedBrowser(t *testing.T) {
	client := &cloak.Client{}
	runtime := &agentruntime.Runtime{Cloak: client}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CallTool(t.Context(), "list_pages", nil, "verify owned cleanup"); !errors.Is(err, cloak.ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestCloakSessionsRealBrowserAcrossTurns(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_CLOAK") != "1" {
		t.Skip("set SOLOMON_TEST_CLOAK=1 for the installed browser")
	}
	configPath := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(configPath, []byte("{\"mcpServers\":{}}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOLOMON_MCP_CONFIG", configPath)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>Session fixture</title><button onclick=\"document.querySelector('p').textContent='Clicked across turns'\">Continue</button><p>Ready</p>")
	}))
	defer server.Close()
	sessions := cloak.NewSessions(t.Context(), nil)
	defer sessions.Close()
	first, err := sessions.Get("project", "chat")
	if err != nil {
		t.Fatal(err)
	}
	turnCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	firstRuntime := &agentruntime.Runtime{Cloak: first, CloakShared: true, ProjRoot: t.TempDir()}
	firstRuntime.InitMCP(turnCtx)
	if firstRuntime.Cloak != first {
		t.Fatal("InitMCP replaced session browser")
	}
	opened, err := first.CallTool(turnCtx, "new_page", map[string]any{"url": server.URL}, "open first-turn fixture")
	if err != nil {
		t.Fatal(err)
	}
	pageID := opened["pageId"].(string)
	snap, err := first.CallTool(turnCtx, "take_snapshot", map[string]any{"pageId": pageID}, "capture first-turn controls")
	if err != nil {
		t.Fatal(err)
	}
	uid := ""
	for _, raw := range snap["elements"].([]any) {
		el := raw.(map[string]any)
		if el["tag"] == "button" {
			uid = el["uid"].(string)
		}
	}
	if uid == "" {
		t.Fatal("fixture button not found")
	}
	if err := firstRuntime.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	second, err := sessions.Get("project", "chat")
	if err != nil {
		t.Fatal(err)
	}
	secondRuntime := &agentruntime.Runtime{Cloak: second, CloakShared: true, ProjRoot: firstRuntime.ProjRoot}
	secondRuntime.InitMCP(t.Context())
	defer secondRuntime.Close()
	if secondRuntime.Cloak != first {
		t.Fatal("second turn lost original browser")
	}
	pages, err := second.CallTool(t.Context(), "list_pages", nil, "verify pages after first turn")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages["pages"].([]any)) != 1 {
		t.Fatalf("unexpected pages: %v", pages)
	}
	if _, err := second.CallTool(t.Context(), "click", map[string]any{"pageId": pageID, "uid": uid}, "use original control in second turn"); err != nil {
		t.Fatal(err)
	}
	result, err := second.CallTool(t.Context(), "take_snapshot", map[string]any{"pageId": pageID}, "verify second-turn action")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result["text"].(string), "Clicked across turns") {
		t.Fatal("second-turn action did not reach original page")
	}
}
