package test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
)

type searchMCPToolCallerTestDouble struct {
	server  string
	tool    string
	intent  string
	args    map[string]any
	result  any
	results []any
	err     error
}

func (c *searchMCPToolCallerTestDouble) CallInternalTool(_ context.Context, serverName, toolName, intent string, args map[string]any) (any, error) {
	c.server = serverName
	c.tool = toolName
	c.intent = intent
	c.args = args
	if len(c.results) > 0 {
		result := c.results[0]
		c.results = c.results[1:]
		return result, c.err
	}
	return c.result, c.err
}

func TestExaSearchAdapterMapsHostedSearchTool(t *testing.T) {
	caller := &searchMCPToolCallerTestDouble{result: "Title: Solomon\nURL: https://example.com/solomon\nPublished: 2026-09-01\nAuthor: Oni\nHighlights:\nA useful excerpt."}
	adapter, err := search.NewExaAdapter(caller, "exa-web")
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.Search(context.Background(), search.Request{
		Query:      "Solomon MCP",
		MaxResults: 4,
		Intent:     "find Solomon MCP documentation",
	})
	if err != nil {
		t.Fatal(err)
	}
	if caller.server != "exa-web" || caller.tool != search.ExaSearchToolName || caller.intent != "find Solomon MCP documentation" {
		t.Fatalf("unexpected call: server=%q tool=%q intent=%q", caller.server, caller.tool, caller.intent)
	}
	if caller.args["query"] != "Solomon MCP" || caller.args["numResults"] != 4 {
		t.Fatalf("arguments=%#v", caller.args)
	}
	if response.Metadata == nil || response.Metadata.Provider != search.ExaAdapterName || response.Metadata.Adapter != search.ExaAdapterName {
		t.Fatalf("unexpected metadata: %+v", response.Metadata)
	}
	if len(response.Hits) != 1 || response.Hits[0].URL != "https://example.com/solomon" || response.Hits[0].Snippet != "A useful excerpt." {
		t.Fatalf("unexpected hits: %+v", response.Hits)
	}
}

func TestParallelSearchAdapterMapsRequiredFields(t *testing.T) {
	caller := &searchMCPToolCallerTestDouble{result: map[string]any{
		"search_id":  "search-123",
		"session_id": "session-123",
		"results": []any{
			map[string]any{
				"url":          "https://example.com/parallel",
				"title":        "Parallel result",
				"publish_date": "2026-09-02",
				"excerpts":     []any{"Relevant excerpt."},
			},
			map[string]any{
				"url":   "https://example.com/second",
				"title": "Second result",
			},
		},
	}}
	adapter, err := search.NewParallelAdapter(caller, "parallel-web")
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.Search(context.Background(), search.Request{
		Query:      "latest MCP support in Solomon",
		MaxResults: 1,
		Extras: map[string]any{
			"sessionID": "session-from-request",
			"modelName": "test-model",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if caller.tool != search.ParallelSearchToolName || caller.args["objective"] != "latest MCP support in Solomon" {
		t.Fatalf("call=%#v", caller.args)
	}
	queries, ok := caller.args["search_queries"].([]string)
	if !ok || len(queries) != 1 || len(strings.Fields(queries[0])) > 6 || queries[0] == "" {
		t.Fatalf("search_queries=%#v", caller.args["search_queries"])
	}
	if caller.args["session_id"] != "session-from-request" || caller.args["model_name"] != "test-model" {
		t.Fatalf("optional args=%#v", caller.args)
	}
	if _, exists := caller.args["max_results"]; exists {
		t.Fatalf("max_results must not be sent as an unsupported per-call MCP argument")
	}
	if response.Metadata == nil || response.Metadata.ProviderRequestID != "search-123" || response.Metadata.SessionID != "session-123" {
		t.Fatalf("unexpected metadata: %+v", response.Metadata)
	}
	if len(response.Hits) != 1 || !response.HasMore || response.Hits[0].PublishedAt != "2026-09-02" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestMCPSearchAdaptersRejectEmptyResults(t *testing.T) {
	caller := &searchMCPToolCallerTestDouble{result: map[string]any{"results": []any{}}}
	adapter, err := search.NewParallelAdapter(caller, "parallel-web")
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Search(context.Background(), search.Request{Query: "empty"})
	if !errors.Is(err, search.ErrNoResults) {
		t.Fatalf("error=%v; want ErrNoResults", err)
	}
}

type searchRouterEngineTestDouble struct {
	name      string
	responses []searchRouterResponse
	calls     int
}

type searchRouterResponse struct {
	response search.Response
	err      error
}

func (e *searchRouterEngineTestDouble) Search(_ context.Context, _ search.Request) (search.Response, error) {
	e.calls++
	if len(e.responses) == 0 {
		return search.Response{}, fmt.Errorf("%s exhausted", e.name)
	}
	response := e.responses[0]
	e.responses = e.responses[1:]
	return response.response, response.err
}

func TestSearchRouterAlternatesConfiguredBackends(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	store := search.NewMemoryBalanceStore(search.BalanceState{})
	exa := &searchRouterEngineTestDouble{name: search.ExaAdapterName, responses: []searchRouterResponse{
		{response: search.Response{Hits: []search.Hit{{URL: "https://exa.example/1"}}}},
		{response: search.Response{Hits: []search.Hit{{URL: "https://exa.example/2"}}}},
	}}
	parallel := &searchRouterEngineTestDouble{name: search.ParallelAdapterName, responses: []searchRouterResponse{
		{response: search.Response{Hits: []search.Hit{{URL: "https://parallel.example/1"}}}},
		{response: search.Response{Hits: []search.Hit{{URL: "https://parallel.example/2"}}}},
	}}
	router := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exa,
		search.ParallelAdapterName: parallel,
	}, search.RouterOptions{Store: store, Now: func() time.Time { return month }})

	for i, want := range []string{search.ExaAdapterName, search.ParallelAdapterName, search.ExaAdapterName, search.ParallelAdapterName} {
		response, err := router.Search(context.Background(), search.Request{Query: fmt.Sprintf("query %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if response.Engine != want {
			t.Fatalf("request %d used %q; want %q", i, response.Engine, want)
		}
	}
	state := store.Snapshot()
	if state.Month != "2026-09" || state.Counts[search.ExaAdapterName] != 2 || state.Counts[search.ParallelAdapterName] != 2 {
		t.Fatalf("unexpected balance state: %+v", state)
	}
}

func TestSearchRouterFallsBackAndPrioritizesFailedPrimary(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	store := search.NewMemoryBalanceStore(search.BalanceState{})
	exa := &searchRouterEngineTestDouble{name: search.ExaAdapterName, responses: []searchRouterResponse{
		{err: errors.New("exa unavailable")},
		{response: search.Response{Hits: []search.Hit{{URL: "https://exa.example/recovered"}}}},
	}}
	parallel := &searchRouterEngineTestDouble{name: search.ParallelAdapterName, responses: []searchRouterResponse{
		{response: search.Response{Hits: []search.Hit{{URL: "https://parallel.example/fallback"}}}},
		{response: search.Response{Hits: []search.Hit{{URL: "https://parallel.example/next"}}}},
	}}
	router := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exa,
		search.ParallelAdapterName: parallel,
	}, search.RouterOptions{Store: store, Now: func() time.Time { return month }})

	response, err := router.Search(context.Background(), search.Request{Query: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Engine != search.ParallelAdapterName || response.Metadata == nil || !response.Metadata.Fallback || len(response.Metadata.Attempts) != 2 {
		t.Fatalf("unexpected fallback response: %+v", response)
	}
	state := store.Snapshot()
	if state.Counts[search.ExaAdapterName] != 1 || state.Counts[search.ParallelAdapterName] != 1 || state.Next != search.ExaAdapterName {
		t.Fatalf("unexpected fallback state: %+v", state)
	}
	response, err = router.Search(context.Background(), search.Request{Query: "retry primary"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Engine != search.ExaAdapterName {
		t.Fatalf("next request used %q; want failed primary %q", response.Engine, search.ExaAdapterName)
	}
}

func TestSearchRouterResetsCountsAtMonthBoundary(t *testing.T) {
	current := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	store := search.NewMemoryBalanceStore(search.BalanceState{
		Month:  "2026-08",
		Counts: map[string]int64{search.ExaAdapterName: 99, search.ParallelAdapterName: 2},
		Next:   search.ParallelAdapterName,
	})
	exa := &searchRouterEngineTestDouble{name: search.ExaAdapterName, responses: []searchRouterResponse{{response: search.Response{Hits: []search.Hit{{URL: "https://exa.example/reset"}}}}}}
	parallel := &searchRouterEngineTestDouble{name: search.ParallelAdapterName, responses: []searchRouterResponse{{response: search.Response{Hits: []search.Hit{{URL: "https://parallel.example/reset"}}}}}}
	router := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exa,
		search.ParallelAdapterName: parallel,
	}, search.RouterOptions{Store: store, Now: func() time.Time { return current }})

	response, err := router.Search(context.Background(), search.Request{Query: "new month"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Engine != search.ExaAdapterName {
		t.Fatalf("reset scheduler used %q; want %q", response.Engine, search.ExaAdapterName)
	}
	state := store.Snapshot()
	if state.Month != "2026-09" || state.Counts[search.ExaAdapterName] != 1 || state.Counts[search.ParallelAdapterName] != 0 {
		t.Fatalf("unexpected reset state: %+v", state)
	}
}

func TestSearchRouterUsesNativeFallbackAfterBothPrimaryBackends(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	exa := &searchRouterEngineTestDouble{name: search.ExaAdapterName, responses: []searchRouterResponse{{err: errors.New("exa failed")}}}
	parallel := &searchRouterEngineTestDouble{name: search.ParallelAdapterName, responses: []searchRouterResponse{{err: errors.New("parallel failed")}}}
	fallback := &searchRouterEngineTestDouble{name: search.CloakAdapterName, responses: []searchRouterResponse{{response: search.Response{Hits: []search.Hit{{URL: "https://example.com/native-cloak"}}}}}}
	router := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exa,
		search.ParallelAdapterName: parallel,
	}, search.RouterOptions{
		Store:    search.NewMemoryBalanceStore(search.BalanceState{}),
		Now:      func() time.Time { return month },
		Fallback: fallback,
	})

	response, err := router.Search(context.Background(), search.Request{Query: "native fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Engine != search.CloakAdapterName || response.Metadata == nil || !response.Metadata.Fallback || len(response.Metadata.Attempts) != 3 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if fallback.calls != 1 {
		t.Fatalf("native fallback calls=%d", fallback.calls)
	}
}

func TestSearchRouterReturnsAttemptsWhenAllBackendsFail(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	exa := &searchRouterEngineTestDouble{name: search.ExaAdapterName, responses: []searchRouterResponse{{err: errors.New("exa failed")}}}
	parallel := &searchRouterEngineTestDouble{name: search.ParallelAdapterName, responses: []searchRouterResponse{{err: search.ErrNoResults}}}
	router := search.NewRouter(map[string]search.Engine{
		search.ExaAdapterName:      exa,
		search.ParallelAdapterName: parallel,
	}, search.RouterOptions{Store: search.NewMemoryBalanceStore(search.BalanceState{}), Now: func() time.Time { return month }})

	_, err := router.Search(context.Background(), search.Request{Query: "failed"})
	var routerErr *search.RouterError
	if !errors.As(err, &routerErr) || len(routerErr.Attempts) != 2 || !errors.Is(err, search.ErrNoResults) {
		t.Fatalf("unexpected router error: %v", err)
	}
}
