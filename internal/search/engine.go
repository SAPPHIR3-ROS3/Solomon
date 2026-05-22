package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
)

var ErrUnknownEngine = errors.New("unknown search engine")

type Hit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type Response struct {
	Engine       string `json:"engine"`
	Hits         []Hit  `json:"hits"`
	HasMore      bool   `json:"hasMore,omitempty"`
	SearxBaseURL string `json:"searxBaseURL,omitempty"`
}

type Request struct {
	Query      string
	MaxResults int
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
	out, err := e.Search(ctx, req)
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "search engine request failed", logging.LogOptions{Params: map[string]any{"engine": engineKey, "query": req.Query, "err": err.Error()}})
		return Response{}, err
	}
	out.Engine = strings.TrimSpace(strings.ToLower(engineKey))
	return out, nil
}
