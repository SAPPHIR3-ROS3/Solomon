package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

type roleBackendCache struct {
	mu   sync.Mutex
	byID map[string]llm.CompletionBackend
}

func (c *roleBackendCache) get(providerName string) (llm.CompletionBackend, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		return nil, false
	}
	b, ok := c.byID[providerName]
	return b, ok
}

func (c *roleBackendCache) set(providerName string, b llm.CompletionBackend) {
	if c == nil || b == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		c.byID = make(map[string]llm.CompletionBackend)
	}
	c.byID[providerName] = b
}

func (r *Runtime) backendForProvider(ctx context.Context, providerName string) (llm.CompletionBackend, error) {
	if r == nil || r.Cfg == nil {
		return nil, fmt.Errorf("runtime not configured")
	}
	name := strings.TrimSpace(providerName)
	if name == "" {
		return nil, fmt.Errorf("empty provider name")
	}
	if r.Prov != nil && strings.TrimSpace(r.Prov.Name) == name && r.Backend != nil {
		return r.Backend, nil
	}
	if b, ok := r.roleBackends.get(name); ok {
		return b, nil
	}
	p := config.ProviderByName(r.Cfg, name)
	if p == nil {
		return nil, fmt.Errorf("provider %q not found in config", providerName)
	}
	b, err := llm.NewCompletionBackend(ctx, r.Cfg, p)
	if err != nil {
		return nil, err
	}
	r.roleBackends.set(name, b)
	return b, nil
}
