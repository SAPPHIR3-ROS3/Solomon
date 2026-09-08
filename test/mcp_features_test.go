package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

type mcpFeatureCounters struct {
	elicitation     atomic.Int32
	sampling        atomic.Int32
	toolChange      chan struct{}
	promptChange    chan struct{}
	resourcesChange chan struct{}
	resourceUpdate  chan struct{}
}

func newMCPFeatureServer(t *testing.T) (*sdkmcp.Server, *httptest.Server, <-chan struct{}) {
	t.Helper()

	subscriptionReady := make(chan struct{})
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "full-feature-test", Version: "1"}, &sdkmcp.ServerOptions{
		CompletionHandler: func(context.Context, *sdkmcp.CompleteRequest) (*sdkmcp.CompleteResult, error) {
			return &sdkmcp.CompleteResult{Completion: sdkmcp.CompletionResultDetails{
				Values: []string{"alice", "alicia"},
				Total:  2,
			}}, nil
		},
		SubscribeHandler: func(context.Context, *sdkmcp.SubscribeRequest) error {
			return nil
		},
		UnsubscribeHandler: func(context.Context, *sdkmcp.UnsubscribeRequest) error {
			return nil
		},
	})
	server.AddSendingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			result, err := next(ctx, method, req)
			if method == "notifications/subscriptions/acknowledged" {
				select {
				case <-subscriptionReady:
				default:
					close(subscriptionReady)
				}
			}
			return result, err
		}
	})

	server.AddTool(&sdkmcp.Tool{
		Name:        "input",
		Description: "Exercise MCP multi-round-trip input requests",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &sdkmcp.CallToolResult{
				InputRequests: sdkmcp.InputRequestMap{
					"confirm": &sdkmcp.ElicitParams{
						Mode:    "form",
						Message: "Continue?",
						RequestedSchema: map[string]any{
							"type":       "object",
							"properties": map[string]any{"answer": map[string]any{"type": "string"}},
						},
					},
					"roots":  &sdkmcp.ListRootsParams{},
					"sample": &sdkmcp.CreateMessageParams{MaxTokens: 1},
				},
				RequestState: "feature-state",
			}, nil
		}

		confirm, ok := req.Params.InputResponses["confirm"].(*sdkmcp.ElicitResult)
		if !ok || confirm.Action != "accept" || confirm.Content["answer"] != "yes" {
			return nil, fmt.Errorf("unexpected elicitation response: %#v", req.Params.InputResponses["confirm"])
		}
		roots, ok := req.Params.InputResponses["roots"].(*sdkmcp.ListRootsResult)
		if !ok || len(roots.Roots) != 1 || roots.Roots[0].URI != "file:///workspace" {
			return nil, fmt.Errorf("unexpected roots response: %#v", req.Params.InputResponses["roots"])
		}
		sample, ok := req.Params.InputResponses["sample"].(*sdkmcp.CreateMessageWithToolsResult)
		if !ok || len(sample.Content) != 1 {
			return nil, fmt.Errorf("unexpected sampling response: %#v", req.Params.InputResponses["sample"])
		}
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "input-complete"}}}, nil
	})

	server.AddResource(&sdkmcp.Resource{
		URI:      "memo:///one",
		Name:     "one",
		MIMEType: "text/plain",
	}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "text/plain",
			Text:     "resource-content",
		}}}, nil
	})
	server.AddResourceTemplate(&sdkmcp.ResourceTemplate{
		URITemplate: "memo:///{id}",
		Name:        "memo-by-id",
	}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: "template-content"}}}, nil
	})
	server.AddPrompt(&sdkmcp.Prompt{
		Name:        "greet",
		Description: "Greet a person",
		Arguments:   []*sdkmcp.PromptArgument{{Name: "name", Required: true}},
	}, func(_ context.Context, req *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{Messages: []*sdkmcp.PromptMessage{{
			Role:    "user",
			Content: &sdkmcp.TextContent{Text: "hello " + req.Params.Arguments["name"]},
		}}}, nil
	})

	handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true, PropagateRequestCancellation: true})
	return server, httptest.NewServer(handler), subscriptionReady
}

func waitForMCPFeatureSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for MCP %s notification", what)
	}
}

func waitForMCPFeatureCondition(t *testing.T, condition func() bool, what string) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out waiting for MCP %s catalog refresh", what)
		case <-ticker.C:
		}
	}
}

func TestMCPManagerSupportsCoreJulyFeatures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	counters := &mcpFeatureCounters{
		toolChange:      make(chan struct{}, 2),
		promptChange:    make(chan struct{}, 2),
		resourcesChange: make(chan struct{}, 2),
		resourceUpdate:  make(chan struct{}, 2),
	}
	server, httpServer, subscriptionReady := newMCPFeatureServer(t)
	t.Cleanup(httpServer.Close)

	options := &mcp.ManagerOptions{
		Roots: []*sdkmcp.Root{{URI: "file:///workspace", Name: "workspace"}},
		ClientOptions: sdkmcp.ClientOptions{
			ElicitationHandler: func(context.Context, *sdkmcp.ElicitRequest) (*sdkmcp.ElicitResult, error) {
				counters.elicitation.Add(1)
				return &sdkmcp.ElicitResult{Action: "accept", Content: map[string]any{"answer": "yes"}}, nil
			},
			CreateMessageHandler: func(context.Context, *sdkmcp.CreateMessageRequest) (*sdkmcp.CreateMessageResult, error) {
				counters.sampling.Add(1)
				return &sdkmcp.CreateMessageResult{
					Content: &sdkmcp.TextContent{Text: "sampled"},
					Model:   "test-model",
					Role:    "assistant",
				}, nil
			},
			ToolListChangedHandler: func(context.Context, *sdkmcp.ToolListChangedRequest) {
				counters.toolChange <- struct{}{}
			},
			PromptListChangedHandler: func(context.Context, *sdkmcp.PromptListChangedRequest) {
				counters.promptChange <- struct{}{}
			},
			ResourceListChangedHandler: func(context.Context, *sdkmcp.ResourceListChangedRequest) {
				counters.resourcesChange <- struct{}{}
			},
			ResourceUpdatedHandler: func(context.Context, *sdkmcp.ResourceUpdatedNotificationRequest) {
				counters.resourceUpdate <- struct{}{}
			},
		},
	}

	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name: "features",
		Type: mcp.TransportStreamableHTTP,
		URL:  httpServer.URL,
	}}}, nil, options)
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Errorf("close MCP manager: %v", err)
		}
	})

	servers := mgr.Servers()
	if len(servers) != 1 || servers[0].ProtocolVersion != "2026-07-28" {
		t.Fatalf("MCP server negotiation = %#v, want one July 2026 server", servers)
	}
	if servers[0].Implementation == nil || servers[0].Implementation.Name != "full-feature-test" {
		t.Fatalf("MCP server implementation = %#v", servers[0].Implementation)
	}
	if servers[0].Capabilities == nil || servers[0].Capabilities.Tools == nil || servers[0].Capabilities.Resources == nil || servers[0].Capabilities.Prompts == nil || servers[0].Capabilities.Completions == nil {
		t.Fatalf("MCP server capabilities = %#v, missing core capability", servers[0].Capabilities)
	}

	if !mgr.HasTool("MCP.features.input") {
		t.Fatalf("MCP input tool was not registered: %#v", mgr.Catalog())
	}
	tools := mgr.Tools()
	if len(tools) != 1 || tools[0].Definition == nil || tools[0].Definition.Name != "input" {
		t.Fatalf("complete MCP tool catalog = %#v", tools)
	}
	if len(mgr.Resources()) != 1 || len(mgr.ResourceTemplates()) != 1 || len(mgr.Prompts()) != 1 {
		t.Fatalf("MCP catalogs = resources %d, templates %d, prompts %d", len(mgr.Resources()), len(mgr.ResourceTemplates()), len(mgr.Prompts()))
	}

	resources, err := mgr.ListResources(ctx, "features")
	if err != nil || len(resources) != 1 || resources[0].URI != "memo:///one" {
		t.Fatalf("ListResources() = %#v, %v", resources, err)
	}
	templates, err := mgr.ListResourceTemplates(ctx, "features")
	if err != nil || len(templates) != 1 || templates[0].URITemplate != "memo:///{id}" {
		t.Fatalf("ListResourceTemplates() = %#v, %v", templates, err)
	}
	read, err := mgr.ReadResource(ctx, "features", "memo:///one")
	if err != nil || len(read.Contents) != 1 || read.Contents[0].Text != "resource-content" {
		t.Fatalf("ReadResource() = %#v, %v", read, err)
	}
	prompts, err := mgr.ListPrompts(ctx, "features")
	if err != nil || len(prompts) != 1 || prompts[0].Name != "greet" {
		t.Fatalf("ListPrompts() = %#v, %v", prompts, err)
	}
	prompt, err := mgr.GetPrompt(ctx, "features", "greet", map[string]string{"name": "Ada"})
	if err != nil || len(prompt.Messages) != 1 || prompt.Messages[0].Content.(*sdkmcp.TextContent).Text != "hello Ada" {
		t.Fatalf("GetPrompt() = %#v, %v", prompt, err)
	}
	completion, err := mgr.Complete(ctx, "features", &sdkmcp.CompleteParams{
		Ref:      &sdkmcp.CompleteReference{Type: "ref/prompt", Name: "greet"},
		Argument: sdkmcp.CompleteParamsArgument{Name: "name", Value: "al"},
	})
	if err != nil || len(completion.Completion.Values) != 2 {
		t.Fatalf("Complete() = %#v, %v", completion, err)
	}
	if err := mgr.Ping(ctx, "features"); err == nil || !strings.Contains(err.Error(), "does not support ping") {
		t.Fatalf("Ping() = %v, want July protocol compatibility error", err)
	}

	result, err := mgr.CallTool(ctx, "MCP.features.input", json.RawMessage(`{"intent":"exercise all July MCP input requests"}`))
	if err != nil || result != "input-complete" {
		t.Fatalf("CallTool() = %#v, %v", result, err)
	}
	if counters.elicitation.Load() != 1 || counters.sampling.Load() != 1 {
		t.Fatalf("input handlers = elicitation %d, sampling %d", counters.elicitation.Load(), counters.sampling.Load())
	}

	if err := mgr.Subscribe(ctx, "features", "memo:///one"); err != nil {
		t.Fatalf("Subscribe() = %v", err)
	}
	if err := server.ResourceUpdated(ctx, &sdkmcp.ResourceUpdatedNotificationParams{URI: "memo:///one"}); err != nil {
		t.Fatalf("ResourceUpdated() = %v", err)
	}
	waitForMCPFeatureSignal(t, counters.resourceUpdate, "resource update")
	if err := mgr.Unsubscribe(ctx, "features", "memo:///one"); err != nil {
		t.Fatalf("Unsubscribe() = %v", err)
	}
	if err := mgr.SetLoggingLevel(ctx, "features", sdkmcp.LoggingLevel("info")); err == nil || !strings.Contains(err.Error(), "does not support logging level") {
		t.Fatalf("SetLoggingLevel() = %v, want July protocol compatibility error", err)
	}

	waitForMCPFeatureSignal(t, subscriptionReady, "modern subscription listener")
	server.AddTool(&sdkmcp.Tool{Name: "late", Description: "late tool", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "late"}}}, nil
	})
	waitForMCPFeatureSignal(t, counters.toolChange, "tool list")
	waitForMCPFeatureCondition(t, func() bool { return mgr.HasTool("MCP.features.late") }, "tool list")
	if !mgr.HasTool("MCP.features.late") {
		t.Fatalf("late MCP tool was not added: %#v", mgr.Catalog())
	}

	server.AddPrompt(&sdkmcp.Prompt{Name: "late-prompt"}, func(context.Context, *sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		return &sdkmcp.GetPromptResult{}, nil
	})
	waitForMCPFeatureSignal(t, counters.promptChange, "prompt list")
	waitForMCPFeatureCondition(t, func() bool { return len(mgr.Prompts()) == 2 }, "prompt list")
	if got := len(mgr.Prompts()); got != 2 {
		t.Fatalf("prompt catalog size = %d, want 2", got)
	}

	server.AddResource(&sdkmcp.Resource{URI: "memo:///two", Name: "two"}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, Text: "two"}}}, nil
	})
	waitForMCPFeatureSignal(t, counters.resourcesChange, "resource list")
	waitForMCPFeatureCondition(t, func() bool { return len(mgr.Resources()) == 2 }, "resource list")
	if got := len(mgr.Resources()); got != 2 {
		t.Fatalf("resource catalog size = %d, want 2", got)
	}
}

type staticMCPAuthHandler struct{}

func (*staticMCPAuthHandler) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "mcp-test-token"}), nil
}

func (*staticMCPAuthHandler) Authorize(context.Context, *http.Request, *http.Response) error {
	return errors.New("unexpected OAuth authorization flow")
}

func TestMCPManagerInjectsOAuthWithoutEmbeddingCredentials(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "oauth-test", Version: "1"}, nil)
	server.AddTool(&sdkmcp.Tool{Name: "private", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "authorized"}}}, nil
	})
	mcpHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{Stateless: true})
	authorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp-test-token" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(authorized.Close)

	var factoryCalls atomic.Int32
	mgr := mcp.NewManagerWithOptions(ctx, &mcp.Config{Servers: []mcp.ServerConfig{{
		Name:  "private",
		Type:  mcp.TransportStreamableHTTP,
		URL:   authorized.URL,
		OAuth: &mcp.OAuthConfig{ClientID: "configured-outside-the-build"},
	}}}, nil, &mcp.ManagerOptions{
		OAuthHandler: func(context.Context, mcp.ServerConfig) (auth.OAuthHandler, error) {
			factoryCalls.Add(1)
			return &staticMCPAuthHandler{}, nil
		},
	})
	t.Cleanup(func() {
		if err := mgr.Close(); err != nil {
			t.Errorf("close OAuth MCP manager: %v", err)
		}
	})

	if factoryCalls.Load() != 1 {
		t.Fatalf("OAuth handler factory calls = %d, want 1", factoryCalls.Load())
	}
	result, err := mgr.CallTool(ctx, "MCP.private.private", json.RawMessage(`{"intent":"verify OAuth transport"}`))
	if err != nil || !strings.Contains(fmt.Sprint(result), "authorized") {
		t.Fatalf("authorized CallTool() = %#v, %v", result, err)
	}
}
