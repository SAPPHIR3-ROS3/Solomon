package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type blockedResearchBackend struct {
	surfaceResearchBackend
	once    sync.Once
	entered chan struct{}
	proceed chan struct{}
}

func (b *blockedResearchBackend) CompleteText(ctx context.Context, req llm.SimpleCompletionRequest) (string, error) {
	b.once.Do(func() { close(b.entered); <-b.proceed })
	return b.surfaceResearchBackend.CompleteText(ctx, req)
}

func TestResearchWebAdaptersSurviveParentRuntimeClose(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	var searches, fetches atomic.Int32
	mcp := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "research", Version: "1"}, nil)
	addSurfaceMCPTool(mcp, search.ExaSearchToolName, `{"results":[{"title":"Evidence","url":"https://example.com/research","highlights":["Useful research evidence"]}]}`, &searches)
	addSurfaceMCPTool(mcp, webfetch.ExaFetchToolName, "# Evidence\nURL: https://example.com/research\n\nThe source provides detailed evidence about the research question and supports a useful answer.", &fetches)
	remote := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return mcp }, &sdkmcp.StreamableHTTPOptions{Stateless: true}))
	defer remote.Close()
	body := fmt.Sprintf(`{"mcpServers":{"exa-web":{"type":"streamable-http","url":%q,"internal":true,"adapter":"exa"}}}`, remote.URL)
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	provider := &config.Provider{Name: "test", BaseURL: "http://127.0.0.1:9", APIKey: "key", AuthKind: config.AuthKindAPIKey}
	cfg := &config.Root{Current: config.Current{Provider: "test", Model: "research-test"}, Providers: map[string]*config.Provider{"test": provider}, WebSearchEngine: search.InternalEngineName, ResearchMaxRounds: 1, ResearchMaxURLsPerRound: 1, ResearchMaxContentChars: 2000}
	backend := &blockedResearchBackend{entered: make(chan struct{}), proceed: make(chan struct{})}
	rt := agentruntime.NewTestRuntime(cfg, provider, testProjectHex, t.TempDir(), &chatstore.Session{ID: "research-lifecycle"}, io.Discard)
	rt.Backend = backend
	rt.InitMCP(context.Background())
	defer rt.Close()
	rec, err := rt.StartResearchJob("Which evidence supports the claim?", "factcheck")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.DeleteResearch(rec.ID)
	select {
	case <-backend.entered:
	case <-time.After(5 * time.Second):
		close(backend.proceed)
		t.Fatal("research did not start")
	}
	if err := rt.Close(); err != nil {
		close(backend.proceed)
		t.Fatal(err)
	}
	close(backend.proceed)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rec, err = rt.ResearchStatus(rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Status != research.StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec.Status != research.StatusDone {
		t.Fatalf("research after parent close: status=%s error=%s", rec.Status, rec.Error)
	}
	if searches.Load() == 0 || fetches.Load() == 0 {
		t.Fatalf("adapters not used: searches=%d fetches=%d", searches.Load(), fetches.Load())
	}
}
