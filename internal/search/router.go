package search

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

// MCPAdapterServer identifies one internal MCP server without exposing its
// transport, headers, environment, or other configuration secrets.
type MCPAdapterServer struct {
	ServerName string
	Adapter    string
}

type RouterOptions struct {
	Store    BalanceStore
	Now      func() time.Time
	Fallback Engine
}

// Router balances requests among configured web-search adapters. A backend
// attempt is reserved before the call, so failures are counted just like
// successful requests and the monthly usage stays balanced across fallbacks.
type Router struct {
	adapters      map[string]Engine
	primaryOrder  []string
	fallbackOrder []string
	store         BalanceStore
	volatile      *MemoryBalanceStore
	now           func() time.Time
}

// RouterError reports that every selected backend failed. Its Unwrap method
// preserves errors.Is checks such as errors.Is(err, ErrNoResults).
type RouterError struct {
	Attempts []BackendAttempt
	Err      error
}

func (e *RouterError) Error() string {
	if e == nil || e.Err == nil {
		return "web search backends failed"
	}
	return "web search backends failed: " + e.Err.Error()
}

func (e *RouterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewRouter(adapters map[string]Engine, options RouterOptions) *Router {
	normalized := make(map[string]Engine, len(adapters))
	for name, adapter := range adapters {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" || adapter == nil {
			continue
		}
		normalized[name] = adapter
	}
	if options.Fallback != nil {
		normalized[CloakAdapterName] = options.Fallback
	}
	order := make([]string, 0, len(normalized))
	for name := range normalized {
		order = append(order, name)
	}
	sort.Strings(order)
	order = preferredAdapterOrder(order)
	primaryOrder := make([]string, 0, len(order))
	fallbackOrder := make([]string, 0, len(order))
	for _, name := range order {
		if name == ExaAdapterName || name == ParallelAdapterName {
			primaryOrder = append(primaryOrder, name)
		} else {
			fallbackOrder = append(fallbackOrder, name)
		}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	store := options.Store
	if store == nil {
		store = DefaultBalanceStore()
	}
	return &Router{
		adapters:      normalized,
		primaryOrder:  primaryOrder,
		fallbackOrder: fallbackOrder,
		store:         store,
		volatile:      NewMemoryBalanceStore(BalanceState{}),
		now:           now,
	}
}

// NewMCPRouter creates only the adapters Solomon knows how to host. Unknown
// internal adapter labels are ignored and logged so one optional backend does
// not disable another valid backend from the same mcp.json.
func NewMCPRouter(caller MCPToolCaller, servers []MCPAdapterServer, options RouterOptions) (*Router, error) {
	adapters := make(map[string]Engine)
	seenServers := make(map[string]bool)
	for _, spec := range servers {
		serverName := strings.TrimSpace(spec.ServerName)
		adapterName := strings.TrimSpace(strings.ToLower(spec.Adapter))
		if serverName == "" || adapterName == "" {
			continue
		}
		if seenServers[serverName] {
			logging.Log(logging.WARNING_LOG_LEVEL, "duplicate internal web-search MCP server ignored", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		seenServers[serverName] = true
		if _, exists := adapters[adapterName]; exists {
			logging.Log(logging.WARNING_LOG_LEVEL, "duplicate internal web-search adapter ignored", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		var (
			engine Engine
			err    error
		)
		switch adapterName {
		case ExaAdapterName:
			engine, err = NewExaAdapter(caller, serverName)
		case ParallelAdapterName:
			engine, err = NewParallelAdapter(caller, serverName)
		case CloakAdapterName:
			logging.Log(logging.INFO_LOG_LEVEL, "legacy Cloak MCP adapter ignored; use the native fallback", logging.LogOptions{Params: map[string]any{"server": serverName}})
			continue
		default:
			logging.Log(logging.WARNING_LOG_LEVEL, "unknown internal web-search adapter", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		if err != nil {
			return nil, err
		}
		adapters[adapterName] = engine
	}
	return NewRouter(adapters, options), nil
}

func (r *Router) Search(ctx context.Context, req Request) (Response, error) {
	if r == nil {
		return Response{}, errors.New("web search router unavailable")
	}
	if len(r.adapters) == 0 {
		return Response{}, errors.New("web search router has no configured adapters")
	}
	primary, err := r.reservePrimary()
	if err != nil {
		return Response{}, err
	}
	order := r.attemptOrder(primary)
	attempts := make([]BackendAttempt, 0, len(order))
	errs := make([]error, 0, len(order))
	for index, backend := range order {
		if index > 0 {
			if err := r.reserveAttempt(backend); err != nil {
				return Response{}, err
			}
		}
		started := time.Now()
		response, callErr := RunEngine(ctx, backend, r.adapters[backend], req)
		attempt := BackendAttempt{Backend: backend, Success: callErr == nil, DurationMs: time.Since(started).Milliseconds()}
		if callErr != nil {
			attempt.Error = callErr.Error()
			attempts = append(attempts, attempt)
			errs = append(errs, fmt.Errorf("%s: %w", backend, callErr))
			logging.Log(logging.WARNING_LOG_LEVEL, "web search backend failed", logging.LogOptions{Params: map[string]any{
				"backend": backend, "query": req.Query, "fallback": index > 0, "err": callErr.Error(),
			}})
			continue
		}
		attempts = append(attempts, attempt)
		response.Engine = backend
		metadata := response.Metadata
		if metadata == nil {
			metadata = &ResponseMetadata{}
		}
		if metadata.Provider == "" {
			metadata.Provider = backend
		}
		if metadata.Adapter == "" {
			metadata.Adapter = backend
		}
		metadata.Fallback = index > 0
		metadata.Attempts = append(metadata.Attempts, attempts...)
		response.Metadata = metadata
		r.recordNext(backend, primary, index > 0)
		logging.Log(logging.INFO_LOG_LEVEL, "web search backend completed", logging.LogOptions{Params: map[string]any{
			"backend": backend, "query": req.Query, "fallback": index > 0, "attempts": len(attempts),
		}})
		return response, nil
	}
	r.recordNext(primary, primary, true)
	return Response{}, &RouterError{Attempts: attempts, Err: errors.Join(errs...)}
}

func (r *Router) reservePrimary() (string, error) {
	var selected string
	month := r.currentMonth()
	_, err := r.store.Update(func(state *BalanceState) error {
		normalizeBalanceState(state, month)
		selected = chooseBackend(*state, r.primaryOrder)
		if selected == "" && len(r.fallbackOrder) > 0 {
			selected = r.fallbackOrder[0]
		}
		if selected == "" {
			return errors.New("web search router has no configured adapters")
		}
		if r.isPrimary(selected) {
			state.Counts[selected]++
		}
		return nil
	})
	if err == nil {
		return selected, nil
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "web search balance persistence failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	return r.reserveVolatile(month, "")
}

func (r *Router) reserveAttempt(backend string) error {
	if !r.isPrimary(backend) {
		return nil
	}
	month := r.currentMonth()
	_, err := r.store.Update(func(state *BalanceState) error {
		normalizeBalanceState(state, month)
		state.Counts[backend]++
		return nil
	})
	if err == nil {
		return nil
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "web search balance persistence failed", logging.LogOptions{Params: map[string]any{"backend": backend, "err": err.Error()}})
	_, fallbackErr := r.reserveVolatile(month, backend)
	return fallbackErr
}

func (r *Router) reserveVolatile(month, preferred string) (string, error) {
	var selected string
	_, err := r.volatile.Update(func(state *BalanceState) error {
		normalizeBalanceState(state, month)
		if preferred != "" {
			selected = preferred
		} else {
			selected = chooseBackend(*state, r.primaryOrder)
			if selected == "" && len(r.fallbackOrder) > 0 {
				selected = r.fallbackOrder[0]
			}
		}
		if selected == "" {
			return errors.New("web search router has no configured adapters")
		}
		if r.isPrimary(selected) {
			state.Counts[selected]++
		}
		return nil
	})
	return selected, err
}

func (r *Router) recordNext(backend, primary string, usedFallback bool) {
	next := ""
	if usedFallback && len(r.primaryOrder) > 0 {
		next = primary
	} else {
		for _, candidate := range r.primaryOrder {
			if candidate != backend {
				next = candidate
				break
			}
		}
	}
	month := r.currentMonth()
	_, err := r.store.Update(func(state *BalanceState) error {
		normalizeBalanceState(state, month)
		state.Next = next
		return nil
	})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "web search balance preference persistence failed", logging.LogOptions{Params: map[string]any{"backend": backend, "err": err.Error()}})
		_, _ = r.volatile.Update(func(state *BalanceState) error {
			normalizeBalanceState(state, month)
			state.Next = next
			return nil
		})
	}
}

func (r *Router) attemptOrder(primary string) []string {
	out := make([]string, 0, len(r.primaryOrder)+len(r.fallbackOrder))
	if _, ok := r.adapters[primary]; ok {
		out = append(out, primary)
	}
	for _, backend := range r.primaryOrder {
		if backend != primary {
			out = append(out, backend)
		}
	}
	out = append(out, r.fallbackOrder...)
	return out
}

func (r *Router) isPrimary(backend string) bool {
	for _, name := range r.primaryOrder {
		if name == backend {
			return true
		}
	}
	return false
}

func (r *Router) currentMonth() string {
	return r.now().UTC().Format("2006-01")
}

func normalizeBalanceState(state *BalanceState, month string) {
	if state.Counts == nil {
		state.Counts = map[string]int64{}
	}
	if state.Month != month {
		state.Month = month
		state.Counts = map[string]int64{}
		state.Next = ""
	}
	for name, count := range state.Counts {
		if count < 0 {
			state.Counts[name] = 0
		}
	}
}

func chooseBackend(state BalanceState, order []string) string {
	var selected string
	var minimum int64
	for _, backend := range order {
		count := state.Counts[backend]
		if selected == "" || count < minimum {
			selected = backend
			minimum = count
		}
	}
	if state.Next != "" {
		for _, backend := range order {
			if backend == state.Next && state.Counts[backend] == minimum {
				return backend
			}
		}
	}
	return selected
}

func preferredAdapterOrder(order []string) []string {
	preferred := []string{ExaAdapterName, ParallelAdapterName}
	out := make([]string, 0, len(order))
	used := make(map[string]bool, len(order))
	for _, name := range preferred {
		for _, candidate := range order {
			if candidate == name {
				out = append(out, candidate)
				used[candidate] = true
			}
		}
	}
	for _, name := range order {
		if !used[name] {
			out = append(out, name)
		}
	}
	return out
}
