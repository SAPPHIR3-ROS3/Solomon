package test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
)

type webfetchMCPToolCallerTestDouble struct {
	server  string
	tool    string
	intent  string
	args    map[string]any
	result  any
	results []any
	calls   []webfetchMCPCall
}

type webfetchMCPCall struct {
	server string
	tool   string
	intent string
	args   map[string]any
}

func (c *webfetchMCPToolCallerTestDouble) CallInternalTool(_ context.Context, serverName, toolName, intent string, args map[string]any) (any, error) {
	c.server = serverName
	c.tool = toolName
	c.intent = intent
	c.args = args
	c.calls = append(c.calls, webfetchMCPCall{server: serverName, tool: toolName, intent: intent, args: args})
	if len(c.results) > 0 {
		result := c.results[0]
		c.results = c.results[1:]
		return result, nil
	}
	return c.result, nil
}

func TestExaWebFetcherMapsHostedFetchTool(t *testing.T) {
	caller := &webfetchMCPToolCallerTestDouble{result: "# Solomon\nURL: https://example.com/solomon\n\nUseful page content."}
	fetcher, err := webfetch.NewExaFetcher(caller, "exa-web")
	if err != nil {
		t.Fatal(err)
	}
	result, err := fetcher.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/solomon", Intent: "read Solomon documentation"})
	if err != nil {
		t.Fatal(err)
	}
	if caller.server != "exa-web" || caller.tool != search.ExaFetchToolName || caller.intent != "read Solomon documentation" {
		t.Fatalf("unexpected call: %#v", caller.calls)
	}
	urls, ok := caller.args["urls"].([]string)
	if !ok || len(urls) != 1 || urls[0] != "https://example.com/solomon" {
		t.Fatalf("urls=%#v", caller.args["urls"])
	}
	maxCharacters, ok := caller.args["maxCharacters"].(int)
	if !ok || maxCharacters <= 0 {
		t.Fatalf("maxCharacters=%#v", caller.args["maxCharacters"])
	}
	if result.Title != "Solomon" || result.Markdown != "Useful page content." || result.Metadata == nil || result.Metadata.Adapter != webfetch.ExaAdapterName {
		t.Fatalf("result=%+v", result)
	}
}

func TestParallelWebFetcherMapsCurrentMCPContract(t *testing.T) {
	caller := &webfetchMCPToolCallerTestDouble{result: map[string]any{
		"results": []any{map[string]any{
			"url":          "https://example.com/parallel",
			"title":        "Parallel page",
			"full_content": "Full markdown content.",
		}},
	}}
	fetcher, err := webfetch.NewParallelFetcher(caller, "parallel-web")
	if err != nil {
		t.Fatal(err)
	}
	result, err := fetcher.Fetch(context.Background(), webfetch.Request{
		URL:    "https://example.com/parallel",
		Intent: "inspect Parallel docs",
		Extras: map[string]any{"sessionID": "session-1", "modelName": "test-model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if caller.tool != webfetch.ParallelFetchToolName || caller.args["full_content"] != true || caller.args["session_id"] != "session-1" || caller.args["model_name"] != "test-model" {
		t.Fatalf("call=%#v", caller.calls)
	}
	if caller.args["objective"] != "inspect Parallel docs" || result.Markdown != "Full markdown content." {
		t.Fatalf("call/result: %#v %+v", caller.args, result)
	}
}

func TestMCPWebFetchersRejectEmptyContent(t *testing.T) {
	caller := &webfetchMCPToolCallerTestDouble{result: "No content found for the provided URL(s)."}
	fetcher, err := webfetch.NewExaFetcher(caller, "exa-web")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fetcher.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/empty"})
	if !errors.Is(err, webfetch.ErrNoContent) {
		t.Fatalf("error=%v; want ErrNoContent", err)
	}
}

type webfetchRouterTestDouble struct {
	name      string
	responses []webfetchRouterResponse
	calls     int
}

type webfetchRouterResponse struct {
	result webfetch.Result
	err    error
}

func (f *webfetchRouterTestDouble) Fetch(_ context.Context, _ webfetch.Request) (webfetch.Result, error) {
	f.calls++
	if len(f.responses) == 0 {
		return webfetch.Result{}, fmt.Errorf("%s exhausted", f.name)
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response.result, response.err
}

func TestWebFetchRouterBalancesAndFallsBack(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	store := search.NewMemoryBalanceStore(search.BalanceState{})
	exa := &webfetchRouterTestDouble{name: webfetch.ExaAdapterName, responses: []webfetchRouterResponse{
		{err: errors.New("exa unavailable")},
		{result: webfetch.Result{URL: "https://example.com/exa", Markdown: "exa recovered"}},
	}}
	parallel := &webfetchRouterTestDouble{name: webfetch.ParallelAdapterName, responses: []webfetchRouterResponse{{result: webfetch.Result{URL: "https://example.com/parallel", Markdown: "parallel fallback"}}}}
	router := webfetch.NewRouter(map[string]webfetch.Fetcher{
		webfetch.ExaAdapterName:      exa,
		webfetch.ParallelAdapterName: parallel,
	}, webfetch.RouterOptions{Store: store, Now: func() time.Time { return month }})

	result, err := router.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/page"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Metadata == nil || !result.Metadata.Fallback || len(result.Metadata.Attempts) != 2 || result.URL != "https://example.com/parallel" {
		t.Fatalf("fallback result=%+v", result)
	}
	state := store.Snapshot()
	if state.Counts[webfetch.ExaAdapterName] != 1 || state.Counts[webfetch.ParallelAdapterName] != 1 || state.Next != webfetch.ExaAdapterName {
		t.Fatalf("balance state=%+v", state)
	}
	result, err = router.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/page-2"})
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://example.com/exa" || exa.calls != 2 {
		t.Fatalf("next result=%+v exa_calls=%d", result, exa.calls)
	}
}

func TestWebFetchRouterUsesNativeFallbackAfterBothPrimaryBackends(t *testing.T) {
	month := time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC)
	exa := &webfetchRouterTestDouble{name: webfetch.ExaAdapterName, responses: []webfetchRouterResponse{{err: errors.New("exa failed")}}}
	parallel := &webfetchRouterTestDouble{name: webfetch.ParallelAdapterName, responses: []webfetchRouterResponse{{err: errors.New("parallel failed")}}}
	fallback := &webfetchRouterTestDouble{name: webfetch.CloakAdapterName, responses: []webfetchRouterResponse{{result: webfetch.Result{URL: "https://example.com/native-cloak", Markdown: "native fallback"}}}}
	router := webfetch.NewRouter(map[string]webfetch.Fetcher{
		webfetch.ExaAdapterName:      exa,
		webfetch.ParallelAdapterName: parallel,
	}, webfetch.RouterOptions{
		Store:    search.NewMemoryBalanceStore(search.BalanceState{}),
		Now:      func() time.Time { return month },
		Fallback: fallback,
	})

	result, err := router.Fetch(context.Background(), webfetch.Request{URL: "https://example.com/page"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Metadata == nil || result.Metadata.Adapter != webfetch.CloakAdapterName || !result.Metadata.Fallback {
		t.Fatalf("result=%+v", result)
	}
	if fallback.calls != 1 {
		t.Fatalf("native fallback calls=%d", fallback.calls)
	}
}
