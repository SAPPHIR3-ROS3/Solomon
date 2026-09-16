package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
	sandboxparent "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/parent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type surfaceSearchOutcome struct {
	response search.Response
	err      error
}

type surfaceSearchBackend struct {
	name     string
	mu       sync.Mutex
	outcomes []surfaceSearchOutcome
	requests []search.Request
}

func (e *surfaceSearchBackend) Search(_ context.Context, req search.Request) (search.Response, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, req)
	if len(e.outcomes) == 0 {
		return search.Response{}, fmt.Errorf("%s exhausted", e.name)
	}
	outcome := e.outcomes[0]
	e.outcomes = e.outcomes[1:]
	return outcome.response, outcome.err
}

func (e *surfaceSearchBackend) requestCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.requests)
}

func (e *surfaceSearchBackend) requestSnapshot() []search.Request {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]search.Request(nil), e.requests...)
}

type surfaceFetchOutcome struct {
	result webfetch.Result
	err    error
}

type surfaceFetchBackend struct {
	name     string
	mu       sync.Mutex
	outcomes []surfaceFetchOutcome
	requests []webfetch.Request
}

func (f *surfaceFetchBackend) Fetch(_ context.Context, req webfetch.Request) (webfetch.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if len(f.outcomes) == 0 {
		return webfetch.Result{}, fmt.Errorf("%s exhausted", f.name)
	}
	outcome := f.outcomes[0]
	f.outcomes = f.outcomes[1:]
	return outcome.result, outcome.err
}

func (f *surfaceFetchBackend) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *surfaceFetchBackend) requestSnapshot() []webfetch.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]webfetch.Request(nil), f.requests...)
}

func surfaceSearchSuccess(name, rawURL string) search.Response {
	return search.Response{Hits: []search.Hit{{Title: name + " result", URL: rawURL, Snippet: name + " excerpt"}}}
}

func surfaceFetchSuccess(name string) webfetch.Result {
	return webfetch.Result{URL: "https://example.com/source", Status: 200, ContentType: "text/html", Title: name + " page", Markdown: name + " content"}
}

func TestTerminalOrchestrateUsesInternalWebAdaptersAcrossBackends(t *testing.T) {
	sandboxparent.CloseGlobal()
	t.Cleanup(sandboxparent.CloseGlobal)

	month := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	exaSearch := &surfaceSearchBackend{name: search.ExaAdapterName, outcomes: []surfaceSearchOutcome{
		{response: surfaceSearchSuccess(search.ExaAdapterName, "https://example.com/exa")},
		{err: errors.New("exa unavailable")},
	}}
	parallelSearch := &surfaceSearchBackend{name: search.ParallelAdapterName, outcomes: []surfaceSearchOutcome{
		{response: surfaceSearchSuccess(search.ParallelAdapterName, "https://example.com/parallel")},
		{err: errors.New("parallel unavailable")},
	}}
	exaFetch := &surfaceFetchBackend{name: webfetch.ExaAdapterName, outcomes: []surfaceFetchOutcome{
		{result: surfaceFetchSuccess(webfetch.ExaAdapterName)},
		{err: errors.New("exa fetch unavailable")},
	}}
	parallelFetch := &surfaceFetchBackend{name: webfetch.ParallelAdapterName, outcomes: []surfaceFetchOutcome{
		{result: surfaceFetchSuccess(webfetch.ParallelAdapterName)},
		{err: errors.New("parallel fetch unavailable")},
	}}
	browser := &cloakBrowserTestDouble{
		navigation: cloak.Navigation{URL: "https://example.com/cloak", Status: 200, ContentType: "text/html"},
		snapshot: cloak.Snapshot{
			URL:         "https://example.com/cloak",
			Title:       "Cloak fallback",
			Status:      200,
			ContentType: "text/html",
			Text:        "Cloak rendered content",
			HTML:        `<!doctype html><html><head><title>Cloak fallback</title></head><body><h1>Cloak fallback</h1><p>Rendered content.</p></body></html>`,
			Links:       []cloak.Link{{Title: "Cloak result", URL: "https://example.com/cloak", Snippet: "Cloak excerpt"}},
		},
	}
	cloakSearch, err := search.NewNativeCloakAdapter(browser)
	if err != nil {
		t.Fatal(err)
	}
	cloakFetch, err := webfetch.NewNativeCloakFetcher(browser)
	if err != nil {
		t.Fatal(err)
	}
	searchRouter := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exaSearch,
		search.ParallelAdapterName: parallelSearch,
	}, search.RouterOptions{
		Store:    search.NewMemoryBalanceStore(search.BalanceState{}),
		Now:      func() time.Time { return month },
		Fallback: cloakSearch,
	})
	fetchRouter := webfetch.NewRouter(map[string]webfetch.Fetcher{
		webfetch.ExaAdapterName:      exaFetch,
		webfetch.ParallelAdapterName: parallelFetch,
	}, webfetch.RouterOptions{
		Store:    search.NewMemoryBalanceStore(search.BalanceState{}),
		Now:      func() time.Time { return month },
		Fallback: cloakFetch,
	})

	source := `package main

import (
	"fmt"
	"sdk"
)

func main() {
	s1, err := sdk.WebSearchInfo("first terminal query", "terminal Exa search")
	if err != nil { panic(err) }
	s2, err := sdk.WebSearchInfo("second terminal query", "terminal Parallel search")
	if err != nil { panic(err) }
	s3, err := sdk.WebSearchInfo("third terminal query", "terminal Cloak fallback search")
	if err != nil { panic(err) }
	f1, err := sdk.FetchWebInfo("https://example.com/one", "terminal Exa fetch")
	if err != nil { panic(err) }
	f2, err := sdk.FetchWebInfo("https://example.com/two", "terminal Parallel fetch")
	if err != nil { panic(err) }
	f3, err := sdk.FetchWebInfo("https://example.com/three", "terminal Cloak fallback fetch")
	if err != nil { panic(err) }
	fmt.Printf("%s,%s,%s;%s,%s,%s", s1.Metadata.Adapter, s2.Metadata.Adapter, s3.Metadata.Adapter, f1.Metadata.Adapter, f2.Metadata.Adapter, f3.Metadata.Adapter)
}
`
	args, err := json.Marshal(map[string]string{"source": source, "intent": "exercise terminal web adapters through orchestrate"})
	if err != nil {
		t.Fatal(err)
	}
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{ToolCalls: []llm.AssistantToolCall{{ID: "orchestrate-web", Name: "orchestrate", Arguments: string(args)}}},
			{Content: "terminal web adapters complete"},
		},
	}
	rt := newTurnLoopRuntime(t, backend, nil, func(rt *agentruntime.Runtime) {
		rt.Cfg.WebSearchEngine = search.InternalEngineName
		rt.WebSearch = searchRouter
		rt.WebFetch = fetchRouter
	})
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if exaSearch.requestCount() != 2 || parallelSearch.requestCount() != 2 {
		t.Fatalf("search backend calls: exa=%d parallel=%d", exaSearch.requestCount(), parallelSearch.requestCount())
	}
	if exaFetch.requestCount() != 2 || parallelFetch.requestCount() != 2 {
		t.Fatalf("fetch backend calls: exa=%d parallel=%d", exaFetch.requestCount(), parallelFetch.requestCount())
	}
	joined := ""
	for _, message := range rt.Session.Messages {
		joined += message.Content
	}
	if !strings.Contains(joined, "exa,parallel,cloak;exa,parallel,cloak") {
		t.Fatalf("orchestrate output does not show all adapters: %s", joined)
	}
}

func TestChatModeUsesNativeWebSearchAndFetch(t *testing.T) {
	exaSearch := &surfaceSearchBackend{
		name:     search.ExaAdapterName,
		outcomes: []surfaceSearchOutcome{{response: surfaceSearchSuccess(search.ExaAdapterName, "https://example.com/chat-source")}},
	}
	exaFetch := &surfaceFetchBackend{
		name:     webfetch.ExaAdapterName,
		outcomes: []surfaceFetchOutcome{{result: surfaceFetchSuccess("chat")}},
	}
	searchRouter := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName: exaSearch,
	}, search.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{})})
	fetchRouter := webfetch.NewRouter(map[string]webfetch.Fetcher{
		webfetch.ExaAdapterName: exaFetch,
	}, webfetch.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{})})
	backend := &turnScriptBackend{
		protocol: llm.ProtocolOpenAI,
		turns: []llm.AssistantTurnResult{
			{ToolCalls: []llm.AssistantToolCall{{ID: "chat-search", Name: "webSearch", Arguments: `{"query":"chat mode search","intent":"chat mode search"}`}}},
			{ToolCalls: []llm.AssistantToolCall{{ID: "chat-fetch", Name: "fetchWeb", Arguments: `{"url":"https://example.com/chat-source","intent":"chat mode fetch"}`}}},
			{Content: "chat mode web tools complete"},
		},
	}
	rt := newTurnLoopRuntime(t, backend, nil, func(rt *agentruntime.Runtime) {
		rt.Mode = "chat"
		rt.Cfg.WebSearchEngine = search.InternalEngineName
		rt.WebSearch = searchRouter
		rt.WebFetch = fetchRouter
	})

	tools, err := agenttools.NativeToolParams("chat")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		if tool.OfFunction != nil {
			names[tool.OfFunction.Function.Name] = true
		}
	}
	if !names["webSearch"] || !names["fetchWeb"] {
		t.Fatalf("chat native tools missing webSearch/fetchWeb: %v", names)
	}
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if exaSearch.requestCount() != 1 || exaFetch.requestCount() != 1 {
		t.Fatalf("chat adapter calls: search=%d fetch=%d", exaSearch.requestCount(), exaFetch.requestCount())
	}
	joined := ""
	for _, message := range rt.Session.Messages {
		joined += message.Content
	}
	if !strings.Contains(joined, "chat-source") || !strings.Contains(joined, "chat content") {
		t.Fatalf("chat native tool results missing from session: %s", joined)
	}
}

type surfaceResearchBackend struct {
	mu      sync.Mutex
	prompts []string
}

func (b *surfaceResearchBackend) Protocol() llm.Protocol { return llm.ProtocolOpenAI }

func (b *surfaceResearchBackend) StreamTurn(context.Context, llm.TurnRequest, io.Writer, llm.StreamOpts) (llm.AssistantTurnResult, error) {
	return llm.AssistantTurnResult{}, errors.New("surface research backend does not stream turns")
}

func (b *surfaceResearchBackend) StreamText(context.Context, llm.SimpleCompletionRequest, io.Writer, llm.StreamOpts) (string, llm.UsageStats, error) {
	return "", llm.UsageStats{}, errors.New("surface research backend does not stream text")
}

func (b *surfaceResearchBackend) CompleteText(_ context.Context, req llm.SimpleCompletionRequest) (string, error) {
	b.mu.Lock()
	b.prompts = append(b.prompts, req.User)
	b.mu.Unlock()
	switch {
	case strings.Contains(req.User, "research strategist"):
		return `{"sub_questions":["Which source supports the claim?"],"key_topics":["evidence"],"success_criteria":"Answer with source-backed evidence."}`, nil
	case strings.Contains(req.User, "Generate 4 focused search queries"):
		return `["internal adapter query"]`, nil
	case strings.Contains(req.User, "extracting research evidence"):
		return `{"summary":"The source provides useful evidence.","evidence":"The adapter path was exercised successfully.","rational":"It directly tests the configured web surface."}`, nil
	case strings.Contains(req.User, "updating an evolving research report"):
		return "Evolving report with adapter-backed evidence.", nil
	case strings.HasPrefix(req.User, "Write a **long"):
		return "# Final report\n\nThe configured web adapters returned source-backed evidence.", nil
	case strings.Contains(req.User, "executive TL;DR"):
		return "The configured web adapters worked.", nil
	default:
		return "", fmt.Errorf("unexpected research prompt: %s", req.User)
	}
}

func (b *surfaceResearchBackend) ListModels(context.Context) ([]string, error) {
	return nil, errors.New("surface research backend does not list models")
}

func TestDeepResearchRuntimeUsesInternalWebAdapters(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	projectRoot := t.TempDir()
	provider := &config.Provider{Name: "test", BaseURL: "http://127.0.0.1:9", APIKey: "key", AuthKind: config.AuthKindAPIKey}
	cfg := &config.Root{
		Current:                 config.Current{Provider: "test", Model: "research-test"},
		Providers:               map[string]*config.Provider{"test": provider},
		WebSearchEngine:         search.InternalEngineName,
		ResearchMaxRounds:       1,
		ResearchMaxURLsPerRound: 1,
		ResearchMaxContentChars: 2000,
	}
	searchBackend := &surfaceSearchBackend{
		name:     "internal",
		outcomes: []surfaceSearchOutcome{{response: surfaceSearchSuccess(search.ExaAdapterName, "https://example.com/research-source")}},
	}
	fetchBackend := &surfaceFetchBackend{
		name:     "internal",
		outcomes: []surfaceFetchOutcome{{result: surfaceFetchSuccess("internal")}},
	}
	llmBackend := &surfaceResearchBackend{}
	rt := agentruntime.NewTestRuntime(cfg, provider, testProjectHex, projectRoot, &chatstore.Session{ID: "deep-research-surface"}, io.Discard)
	rt.Backend = llmBackend
	rt.WebSearch = searchBackend
	rt.WebFetch = fetchBackend

	env := &agenttools.Env{
		Cfg: cfg,
		StartResearch: func(ctx context.Context, query, category string) (research.JobRecord, error) {
			return rt.StartResearchJob(query, category)
		},
	}
	out, err := agenttools.Exec(context.Background(), env, "chat", tooling.Invocation{
		Name: "deepResearch",
		Args: json.RawMessage(`{"query":"Which internal web adapter is active?","category":"factcheck","intent":"start adapter research"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var started struct {
		OK    bool   `json:"ok"`
		JobID string `json:"jobId"`
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &started); err != nil {
		t.Fatal(err)
	}
	if !started.OK || started.JobID == "" {
		t.Fatalf("deepResearch result=%s", raw)
	}
	t.Cleanup(func() {
		if started.JobID != "" {
			_ = rt.DeleteResearch(started.JobID)
		}
	})

	var record research.JobRecord
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		record, err = rt.ResearchStatus(started.JobID)
		if err == nil && record.Status != research.StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if record.Status != research.StatusDone {
		t.Fatalf("research status=%q error=%q", record.Status, record.Error)
	}
	if searchBackend.requestCount() != 1 || fetchBackend.requestCount() != 1 {
		t.Fatalf("research web calls: search=%d fetch=%d", searchBackend.requestCount(), fetchBackend.requestCount())
	}
	searchRequests := searchBackend.requestSnapshot()
	fetchRequests := fetchBackend.requestSnapshot()
	if !strings.HasPrefix(searchRequests[0].Intent, "deep research search:") || !strings.HasPrefix(fetchRequests[0].Intent, "deep research page extraction:") {
		t.Fatalf("research intents: search=%q fetch=%q", searchRequests[0].Intent, fetchRequests[0].Intent)
	}
	if record.Stats.SearchEngine != search.InternalEngineName || len(record.Findings) != 1 {
		t.Fatalf("research stats/findings=%+v findings=%+v", record.Stats, record.Findings)
	}
}

func addSurfaceMCPTool(server *sdkmcp.Server, name, response string, calls *atomic.Int32) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        name,
		Description: name + " test adapter",
		InputSchema: map[string]any{"type": "object"},
	}, func(context.Context, *sdkmcp.CallToolRequest, map[string]any) (*sdkmcp.CallToolResult, struct{}, error) {
		calls.Add(1)
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: response}}}, struct{}{}, nil
	})
}

func surfaceAnthropicToolSSE(name, id string, args map[string]any) string {
	raw, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "tool_use", "id": id, "name": name}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": string(raw)}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "tool_use"}},
	)
}

func surfaceAnthropicFinalSSE(text string) string {
	return anthropicSSEBody(
		map[string]any{"type": "message_start", "message": map[string]any{"usage": map[string]any{"input_tokens": 1}}},
		map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text}},
		map[string]any{"type": "message_delta", "usage": map[string]any{"output_tokens": 1}, "delta": map[string]any{"stop_reason": "end_turn"}},
	)
}

func surfaceWebOrchestrateSource(kind, rawURL, intent string) string {
	if kind == "search" {
		return fmt.Sprintf(`package main

import (
	"fmt"
	"sdk"
)

func main() {
	result, err := sdk.WebSearchInfo("%s", "%s")
	if err != nil { panic(err) }
	fmt.Print(result.Metadata.Adapter)
}
`, rawURL, intent)
	}
	return fmt.Sprintf(`package main

import (
	"fmt"
	"sdk"
)

func main() {
	result, err := sdk.FetchWebInfo("%s", "%s")
	if err != nil { panic(err) }
	fmt.Print(result.Metadata.Adapter)
}
`, rawURL, intent)
}

func sendSurfaceGUIMessage(t *testing.T, serverURL, projectID, chatID string, content string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, serverURL+"/__solomon/projects/"+projectID+"/chats/"+chatID+"/messages", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	// The first GUI request may initialize the MCP adapters and compile the
	// orchestrated turn. Keep the test bounded, but leave enough time for a
	// loaded Linux CI runner to complete that setup.
	response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GUI message status=%d body=%s", response.StatusCode, responseBody)
	}
	return responseBody
}

func TestGUIChatUsesInternalExaParallelAdaptersThroughOrchestrate(t *testing.T) {
	sandboxparent.CloseGlobal()
	t.Cleanup(sandboxparent.CloseGlobal)
	var (
		exaSearchCalls      atomic.Int32
		parallelSearchCalls atomic.Int32
		exaFetchCalls       atomic.Int32
		parallelFetchCalls  atomic.Int32
		providerCalls       atomic.Int32
	)
	exaMCP := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "exa-surface", Version: "1"}, nil)
	addSurfaceMCPTool(exaMCP, search.ExaSearchToolName, `{"results":[{"title":"Exa GUI result","url":"https://example.com/exa-gui","highlights":["Exa GUI excerpt"]}]}`, &exaSearchCalls)
	addSurfaceMCPTool(exaMCP, webfetch.ExaFetchToolName, "# Exa GUI\nURL: https://example.com/exa-gui\n\nExa GUI content.", &exaFetchCalls)
	exaHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return exaMCP }, &sdkmcp.StreamableHTTPOptions{Stateless: true}))
	t.Cleanup(exaHTTP.Close)

	parallelMCP := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "parallel-surface", Version: "1"}, nil)
	addSurfaceMCPTool(parallelMCP, search.ParallelSearchToolName, `{"results":[{"title":"Parallel GUI result","url":"https://example.com/parallel-gui","excerpts":["Parallel GUI excerpt"]}]}`, &parallelSearchCalls)
	addSurfaceMCPTool(parallelMCP, webfetch.ParallelFetchToolName, `{"results":[{"url":"https://example.com/parallel-gui","title":"Parallel GUI page","full_content":"Parallel GUI content."}]}`, &parallelFetchCalls)
	parallelHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return parallelMCP }, &sdkmcp.StreamableHTTPOptions{Stateless: true}))
	t.Cleanup(parallelHTTP.Close)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		if len(bytes.TrimSpace(body)) == 0 {
			// A cancelled/deferred title request can arrive after the chat run
			// has completed. It is outside the surface under test.
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"content": []map[string]any{{"type": "text", "text": "GUI title"}}})
			return
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		stream, _ := request["stream"].(bool)
		if !stream {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"content": []map[string]any{{"type": "text", "text": "GUI title"}}})
			return
		}
		call := providerCalls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if call%2 == 0 {
			_, _ = io.WriteString(w, surfaceAnthropicFinalSSE(fmt.Sprintf("GUI adapter run %d complete", call/2)))
			return
		}
		action := (call - 1) / 2
		switch action {
		case 0:
			_, _ = io.WriteString(w, surfaceAnthropicToolSSE("orchestrate", "gui-orchestrate-1", map[string]any{
				"source": surfaceWebOrchestrateSource("search", "GUI Exa search", "GUI Exa search"),
				"intent": "GUI Exa search through orchestrate",
			}))
		case 1:
			_, _ = io.WriteString(w, surfaceAnthropicToolSSE("orchestrate", "gui-orchestrate-2", map[string]any{
				"source": surfaceWebOrchestrateSource("search", "GUI Parallel search", "GUI Parallel search"),
				"intent": "GUI Parallel search through orchestrate",
			}))
		case 2:
			_, _ = io.WriteString(w, surfaceAnthropicToolSSE("orchestrate", "gui-orchestrate-3", map[string]any{
				"source": surfaceWebOrchestrateSource("fetch", "https://example.com/exa-gui", "GUI Exa fetch"),
				"intent": "GUI Exa fetch through orchestrate",
			}))
		case 3:
			_, _ = io.WriteString(w, surfaceAnthropicToolSSE("orchestrate", "gui-orchestrate-4", map[string]any{
				"source": surfaceWebOrchestrateSource("fetch", "https://example.com/parallel-gui", "GUI Parallel fetch"),
				"intent": "GUI Parallel fetch through orchestrate",
			}))
		default:
			t.Errorf("unexpected streamed provider call %d", call)
			_, _ = io.WriteString(w, surfaceAnthropicFinalSSE("unexpected"))
		}
	}))
	t.Cleanup(provider.Close)

	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	home := os.Getenv("SOLOMON_HOME")
	configBody := fmt.Sprintf(`web_search_engine = "internal"

[current]
provider = "test"
model = "claude-test"

[providers.test]
base_url = %q
api_key = "test-key"
api_protocol = "anthropic"
`, provider.URL)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	mcpBody := fmt.Sprintf(`{"mcpServers":{"exa-web":{"type":"streamable-http","url":%q,"internal":true,"adapter":"exa"},"parallel-web":{"type":"streamable-http","url":%q,"internal":true,"adapter":"parallel"}}}`, exaHTTP.URL, parallelHTTP.URL)
	if err := os.WriteFile(filepath.Join(home, "mcp.json"), []byte(mcpBody), 0o600); err != nil {
		t.Fatal(err)
	}
	projectID, chatID := createServerTestChat(t, server.URL)

	for i, prompt := range []string{"GUI search one", "GUI search two", "GUI fetch one", "GUI fetch two"} {
		body := sendSurfaceGUIMessage(t, server.URL, projectID, chatID, prompt)
		if !bytes.Contains(body, []byte(`"type":"chat_snapshot"`)) {
			t.Fatalf("GUI response %d has no final snapshot: %s", i+1, body)
		}
		wantAdapter := []string{search.ExaAdapterName, search.ParallelAdapterName, webfetch.ExaAdapterName, webfetch.ParallelAdapterName}[i]
		if !bytes.Contains(body, []byte(wantAdapter)) {
			t.Fatalf("GUI response %d does not contain adapter %q: %s", i+1, wantAdapter, body)
		}
	}
	if providerCalls.Load() != 8 {
		t.Fatalf("streamed provider calls=%d, want 8", providerCalls.Load())
	}
	if exaSearchCalls.Load() != 1 || parallelSearchCalls.Load() != 1 || exaFetchCalls.Load() != 1 || parallelFetchCalls.Load() != 1 {
		t.Fatalf("GUI adapter calls: exa search=%d fetch=%d; parallel search=%d fetch=%d", exaSearchCalls.Load(), exaFetchCalls.Load(), parallelSearchCalls.Load(), parallelFetchCalls.Load())
	}
}
