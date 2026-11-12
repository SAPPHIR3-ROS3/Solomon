package webfetch

import (
	"context"
	"errors"
	"time"
)

var ErrNoContent = errors.New("fetch returned no content")

// Request is the provider-neutral fetch request used by native web adapters.
// Intent is kept separate from Extras so every MCP call can use Solomon's
// standard intent-aware logging path.
type Request struct {
	URL            string
	TimeoutSeconds int
	Intent         string
	Extras         map[string]any
}

// Fetcher is the runtime-owned web content fetch surface. FetchURL remains
// available as the legacy direct HTTP implementation during migration.
type Fetcher interface {
	Fetch(context.Context, Request) (Result, error)
}

type Attempt struct {
	Backend    string `json:"backend"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type Metadata struct {
	Provider     string         `json:"provider,omitempty"`
	Adapter      string         `json:"adapter,omitempty"`
	Fallback     bool           `json:"fallback,omitempty"`
	Partial      bool           `json:"partial,omitempty"`
	Attempts     []Attempt      `json:"attempts,omitempty"`
	ProviderData map[string]any `json:"providerData,omitempty"`
}

func withFetchMetadata(result Result, provider string, fallback bool, attempts []Attempt) Result {
	metadata := result.Metadata
	if metadata == nil {
		metadata = &Metadata{}
	}
	if metadata.Provider == "" {
		metadata.Provider = provider
	}
	if metadata.Adapter == "" {
		metadata.Adapter = provider
	}
	metadata.Fallback = fallback
	metadata.Attempts = append(metadata.Attempts, attempts...)
	result.Metadata = metadata
	return result
}

func fetchTimeout(request Request) time.Duration {
	seconds := request.TimeoutSeconds
	if seconds <= 0 {
		seconds = DefaultTimeoutS
	}
	if seconds > MaxTimeoutSecs {
		seconds = MaxTimeoutSecs
	}
	return time.Duration(seconds) * time.Second
}
