package webfetch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
)

type MCPAdapterServer struct {
	ServerName string
	Adapter    string
}

type RouterOptions struct {
	Store    search.BalanceStore
	Now      func() time.Time
	Fallback Fetcher
}

type Router struct {
	adapters      map[string]Fetcher
	primaryOrder  []string
	fallbackOrder []string
	store         search.BalanceStore
	volatile      *search.MemoryBalanceStore
	now           func() time.Time
}

type RouterError struct {
	Attempts []Attempt
	Err      error
}

func (e *RouterError) Error() string {
	if e == nil || e.Err == nil {
		return "web fetch backends failed"
	}
	return "web fetch backends failed: " + e.Err.Error()
}

func (e *RouterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewRouter(adapters map[string]Fetcher, options RouterOptions) *Router {
	normalized := make(map[string]Fetcher, len(adapters))
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
	order = preferredFetchOrder(order)
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
		store = search.DefaultBalanceStore()
	}
	return &Router{
		adapters:      normalized,
		primaryOrder:  primaryOrder,
		fallbackOrder: fallbackOrder,
		store:         store,
		volatile:      search.NewMemoryBalanceStore(search.BalanceState{}),
		now:           now,
	}
}

func NewMCPRouter(caller MCPToolCaller, servers []MCPAdapterServer, options RouterOptions) (*Router, error) {
	adapters := make(map[string]Fetcher)
	seenServers := map[string]bool{}
	for _, spec := range servers {
		serverName := strings.TrimSpace(spec.ServerName)
		adapterName := strings.TrimSpace(strings.ToLower(spec.Adapter))
		if serverName == "" || adapterName == "" {
			continue
		}
		if seenServers[serverName] {
			logging.Log(logging.WARNING_LOG_LEVEL, "duplicate internal web-fetch MCP server ignored", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		seenServers[serverName] = true
		if _, exists := adapters[adapterName]; exists {
			logging.Log(logging.WARNING_LOG_LEVEL, "duplicate internal web-fetch adapter ignored", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		var (
			fetcher Fetcher
			err     error
		)
		switch adapterName {
		case ExaAdapterName:
			fetcher, err = NewExaFetcher(caller, serverName)
		case ParallelAdapterName:
			fetcher, err = NewParallelFetcher(caller, serverName)
		case CloakAdapterName:
			logging.Log(logging.INFO_LOG_LEVEL, "legacy Cloak MCP fetcher ignored; use the native fallback", logging.LogOptions{Params: map[string]any{"server": serverName}})
			continue
		default:
			logging.Log(logging.WARNING_LOG_LEVEL, "unknown internal web-fetch adapter", logging.LogOptions{Params: map[string]any{"server": serverName, "adapter": adapterName}})
			continue
		}
		if err != nil {
			return nil, err
		}
		adapters[adapterName] = fetcher
	}
	return NewRouter(adapters, options), nil
}

func (r *Router) Fetch(ctx context.Context, req Request) (Result, error) {
	if r == nil {
		return Result{}, errors.New("web fetch router unavailable")
	}
	if len(r.adapters) == 0 {
		return Result{}, errors.New("web fetch router has no configured adapters")
	}
	primary, err := r.reservePrimary()
	if err != nil {
		return Result{}, err
	}
	order := r.attemptOrder(primary)
	attempts := make([]Attempt, 0, len(order))
	errs := make([]error, 0, len(order))
	for index, backend := range order {
		if index > 0 {
			if err := r.reserveAttempt(backend); err != nil {
				return Result{}, err
			}
		}
		started := time.Now()
		result, fetchErr := r.adapters[backend].Fetch(ctx, req)
		attempt := Attempt{Backend: backend, Success: fetchErr == nil, DurationMs: time.Since(started).Milliseconds()}
		if fetchErr != nil {
			attempt.Error = fetchErr.Error()
			attempts = append(attempts, attempt)
			errs = append(errs, fmt.Errorf("%s: %w", backend, fetchErr))
			logging.Log(logging.WARNING_LOG_LEVEL, "web fetch backend failed", logging.LogOptions{Params: map[string]any{
				"backend": backend, "url": req.URL, "fallback": index > 0, "err": fetchErr.Error(),
			}})
			continue
		}
		if strings.TrimSpace(result.Markdown) == "" {
			fetchErr = fmt.Errorf("%w: %s", ErrNoContent, backend)
			attempt.Success = false
			attempt.Error = fetchErr.Error()
			attempts = append(attempts, attempt)
			errs = append(errs, fetchErr)
			continue
		}
		attempts = append(attempts, attempt)
		result = withFetchMetadata(result, backend, index > 0, attempts)
		r.recordNext(backend, primary, index > 0)
		logging.Log(logging.INFO_LOG_LEVEL, "web fetch backend completed", logging.LogOptions{Params: map[string]any{
			"backend": backend, "url": req.URL, "fallback": index > 0, "attempts": len(attempts),
		}})
		return result, nil
	}
	r.recordNext(primary, primary, true)
	return Result{}, &RouterError{Attempts: attempts, Err: errors.Join(errs...)}
}

func (r *Router) reservePrimary() (string, error) {
	var selected string
	month := r.currentMonth()
	_, err := r.store.Update(func(state *search.BalanceState) error {
		normalizeFetchBalanceState(state, month)
		selected = chooseFetchBackend(*state, r.primaryOrder)
		if selected == "" && len(r.fallbackOrder) > 0 {
			selected = r.fallbackOrder[0]
		}
		if selected == "" {
			return errors.New("web fetch router has no configured adapters")
		}
		if r.isPrimary(selected) {
			state.Counts[selected]++
		}
		return nil
	})
	if err == nil {
		return selected, nil
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "web fetch balance persistence failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
	return r.reserveVolatile(month, "")
}

func (r *Router) reserveAttempt(backend string) error {
	if !r.isPrimary(backend) {
		return nil
	}
	month := r.currentMonth()
	_, err := r.store.Update(func(state *search.BalanceState) error {
		normalizeFetchBalanceState(state, month)
		state.Counts[backend]++
		return nil
	})
	if err == nil {
		return nil
	}
	logging.Log(logging.WARNING_LOG_LEVEL, "web fetch balance persistence failed", logging.LogOptions{Params: map[string]any{"backend": backend, "err": err.Error()}})
	_, fallbackErr := r.reserveVolatile(month, backend)
	return fallbackErr
}

func (r *Router) reserveVolatile(month, preferred string) (string, error) {
	var selected string
	_, err := r.volatile.Update(func(state *search.BalanceState) error {
		normalizeFetchBalanceState(state, month)
		if preferred != "" {
			selected = preferred
		} else {
			selected = chooseFetchBackend(*state, r.primaryOrder)
			if selected == "" && len(r.fallbackOrder) > 0 {
				selected = r.fallbackOrder[0]
			}
		}
		if selected == "" {
			return errors.New("web fetch router has no configured adapters")
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
	_, err := r.store.Update(func(state *search.BalanceState) error {
		normalizeFetchBalanceState(state, month)
		state.Next = next
		return nil
	})
	if err != nil {
		logging.Log(logging.WARNING_LOG_LEVEL, "web fetch balance preference persistence failed", logging.LogOptions{Params: map[string]any{"backend": backend, "err": err.Error()}})
		_, _ = r.volatile.Update(func(state *search.BalanceState) error {
			normalizeFetchBalanceState(state, month)
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
	return append(out, r.fallbackOrder...)
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

func normalizeFetchBalanceState(state *search.BalanceState, month string) {
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

func chooseFetchBackend(state search.BalanceState, order []string) string {
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

func preferredFetchOrder(order []string) []string {
	preferred := []string{ExaAdapterName, ParallelAdapterName}
	out := make([]string, 0, len(order))
	used := map[string]bool{}
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
