package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

var (
	ErrUnknownEngine = errors.New("unknown search engine")
	ErrNoResults     = errors.New("search returned no results")
)

const InternalEngineName = "internal"

type Hit struct {
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Snippet     string         `json:"snippet,omitempty"`
	Content     string         `json:"content,omitempty"`
	Author      string         `json:"author,omitempty"`
	PublishedAt string         `json:"publishedAt,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Response struct {
	Engine       string            `json:"engine"`
	Hits         []Hit             `json:"hits"`
	HasMore      bool              `json:"hasMore,omitempty"`
	SearxBaseURL string            `json:"searxBaseURL,omitempty"`
	Metadata     *ResponseMetadata `json:"metadata,omitempty"`
}

type ResponseMetadata struct {
	Provider          string           `json:"provider,omitempty"`
	Adapter           string           `json:"adapter,omitempty"`
	SessionID         string           `json:"sessionId,omitempty"`
	ProviderRequestID string           `json:"providerRequestId,omitempty"`
	Warnings          []string         `json:"warnings,omitempty"`
	Fallback          bool             `json:"fallback,omitempty"`
	Partial           bool             `json:"partial,omitempty"`
	Attempts          []BackendAttempt `json:"attempts,omitempty"`
	ProviderData      map[string]any   `json:"providerData,omitempty"`
}

type BackendAttempt struct {
	Backend    string `json:"backend"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type Request struct {
	Query      string
	MaxResults int
	Intent     string
	Extras     map[string]any
}

type Engine interface {
	Search(ctx context.Context, req Request) (Response, error)
}

var (
	regMu sync.RWMutex
	reg   = map[string]Engine{}
)

func Register(name string, e Engine) {
	regMu.Lock()
	defer regMu.Unlock()
	reg[strings.TrimSpace(strings.ToLower(name))] = e
}

func Lookup(name string) (Engine, error) {
	k := strings.TrimSpace(strings.ToLower(name))
	regMu.RLock()
	e, ok := reg[k]
	regMu.RUnlock()
	if !ok || e == nil {
		err := fmt.Errorf("%w: %q", ErrUnknownEngine, name)
		logging.Log(logging.WARNING_LOG_LEVEL, "search engine lookup failed", logging.LogOptions{Params: map[string]any{"engine": name, "err": err.Error()}})
		return nil, err
	}
	return e, nil
}

func Run(ctx context.Context, engineKey string, req Request) (Response, error) {
	e, err := Lookup(engineKey)
	if err != nil {
		return Response{}, err
	}
	return runEngine(ctx, engineKey, e, req, true)
}

// RunEngine executes an already-resolved engine. Runtime-owned engines, such
// as the MCP-backed web-search router, use this path so they do not need to
// be put in the process-global registry.
func RunEngine(ctx context.Context, engineKey string, e Engine, req Request) (Response, error) {
	return runEngine(ctx, engineKey, e, req, false)
}

func runEngine(ctx context.Context, engineKey string, e Engine, req Request, forceEngineName bool) (Response, error) {
	if e == nil {
		err := fmt.Errorf("%w: %q", ErrUnknownEngine, engineKey)
		logging.Log(logging.WARNING_LOG_LEVEL, "search engine unavailable", logging.LogOptions{Params: map[string]any{"engine": engineKey, "err": err.Error()}})
		return Response{}, err
	}
	out, err := e.Search(ctx, req)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "search engine request failed", logging.LogOptions{Params: map[string]any{"engine": engineKey, "query": req.Query, "err": err.Error()}})
		return Response{}, err
	}
	if len(out.Hits) == 0 {
		err := fmt.Errorf("%w: %q", ErrNoResults, engineKey)
		logging.Log(logging.WARNING_LOG_LEVEL, "search engine returned no results", logging.LogOptions{Params: map[string]any{"engine": engineKey, "query": req.Query, "err": err.Error()}})
		return Response{}, err
	}
	if forceEngineName || strings.TrimSpace(out.Engine) == "" {
		out.Engine = strings.TrimSpace(strings.ToLower(engineKey))
	}
	return out, nil
}
