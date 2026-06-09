package commands

import (
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type slashCatalogPrefetch struct {
	key          string
	done         chan struct{}
	mixed        []ListedModel
	byProvider   map[string][]ListedModel
	fetchErr     error
	providerErrs []ProviderCatalogError
}

type ProviderCatalogError struct {
	Provider string
	FullList bool
	Err      error
}

func (e ProviderCatalogError) label() string {
	if e.FullList {
		return e.Provider + " (full list)"
	}
	return e.Provider
}

var slashCatalogCache struct {
	mu     sync.Mutex
	active *slashCatalogPrefetch
}

func slashCatalogCacheKey(cfg *config.Root) (string, error) {
	if cfg == nil {
		return "", nil
	}
	var parts []string
	if p, err := paths.ConfigPath(); err == nil {
		if fi, err := os.Stat(p); err == nil {
			parts = append(parts, fi.ModTime().UTC().Format("20060102150405.999999999"))
		}
	}
	providers := config.ProviderList(cfg)
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	for i := range providers {
		pr := &providers[i]
		parts = append(parts, pr.Name, pr.BaseURL, string(pr.EffectiveAuthKind()), pr.EffectiveAPIProtocol())
	}
	parts = append(parts, cfg.Current.Provider, cfg.Current.Model)
	return strings.Join(parts, "\x00"), nil
}

func InvalidateSlashModelCatalogCache() {
	slashCatalogCache.mu.Lock()
	slashCatalogCache.active = nil
	slashCatalogCache.mu.Unlock()
}

func PrefetchSlashModelCatalog(ctx context.Context, cfg *config.Root, out io.Writer) {
	if cfg == nil || config.NeedsOnboard(cfg) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key, err := slashCatalogCacheKey(cfg)
	if err != nil || key == "" {
		return
	}
	slashCatalogCache.mu.Lock()
	if slashCatalogCache.active != nil && slashCatalogCache.active.key == key {
		slashCatalogCache.mu.Unlock()
		return
	}
	p := &slashCatalogPrefetch{key: key, done: make(chan struct{})}
	slashCatalogCache.active = p
	slashCatalogCache.mu.Unlock()
	go runSlashCatalogPrefetch(ctx, cfg, out, p)
}

func runSlashCatalogPrefetch(ctx context.Context, cfg *config.Root, out io.Writer, p *slashCatalogPrefetch) {
	defer close(p.done)
	mixed, byProv, err, provErrs := buildSlashModelCatalogs(ctx, Deps{Ctx: ctx, Cfg: cfg, Out: out})
	p.mixed = mixed
	p.byProvider = byProv
	p.fetchErr = err
	p.providerErrs = provErrs
}

func buildSlashModelCatalogs(ctx context.Context, d Deps) ([]ListedModel, map[string][]ListedModel, error, []ProviderCatalogError) {
	providers := config.ProviderList(d.Cfg)
	if len(providers) == 0 {
		return nil, nil, nil, nil
	}
	type providerBundle struct {
		flagship []ListedModel
		all      []ListedModel
		err      error
		fullList bool
	}
	results := make([]providerBundle, len(providers))
	var wg sync.WaitGroup
	wg.Add(len(providers))
	for i := range providers {
		i := i
		pp := providers[i]
		go func() {
			defer wg.Done()
			ids, err := connect.ListModelsForProvider(ctx, d.Cfg, &pp)
			if err != nil {
				results[i].err = err
				return
			}
			flagship := make([]ListedModel, len(ids))
			for j, mid := range ids {
				flagship[j] = ListedModel{Prov: pp.Name, Model: mid}
			}
			results[i].flagship = flagship
			allIDs, err := connect.ListModelsForProviderAll(ctx, d.Cfg, &pp)
			if err != nil {
				results[i].err = err
				results[i].fullList = true
				return
			}
			results[i].all = orderListedModelsByProvider(orderListedModelsFromIDs(pp.Name, allIDs), pp.Name)
		}()
	}
	wg.Wait()
	var mixed []ListedModel
	byProv := make(map[string][]ListedModel, len(providers))
	var firstErr error
	var providerErrs []ProviderCatalogError
	for i := range results {
		if results[i].err != nil {
			if firstErr == nil {
				firstErr = results[i].err
			}
			providerErrs = append(providerErrs, ProviderCatalogError{
				Provider: providers[i].Name,
				FullList: results[i].fullList,
				Err:      results[i].err,
			})
			continue
		}
		mixed = append(mixed, results[i].flagship...)
		if len(results[i].all) > 0 {
			byProv[providers[i].Name] = results[i].all
		}
	}
	return mixed, byProv, firstErr, providerErrs
}

func waitSlashCatalogPrefetch(ctx context.Context, cfg *config.Root) *slashCatalogPrefetch {
	key, err := slashCatalogCacheKey(cfg)
	if err != nil || key == "" {
		return nil
	}
	slashCatalogCache.mu.Lock()
	p := slashCatalogCache.active
	slashCatalogCache.mu.Unlock()
	if p == nil || p.key != key {
		return nil
	}
	select {
	case <-p.done:
		return p
	case <-ctx.Done():
		return nil
	}
}

func fetchSlashModelCatalogCached(ctx context.Context, d Deps) ([]ListedModel, map[string][]ListedModel, error) {
	if p := waitSlashCatalogPrefetch(ctx, d.Cfg); p != nil {
		return append([]ListedModel(nil), p.mixed...), cloneProviderCatalogMap(p.byProvider), p.fetchErr
	}
	mixed, byProv, err, provErrs := buildSlashModelCatalogs(ctx, d)
	if d.Out != nil && len(provErrs) > 0 {
		PrintProviderCatalogErrors(d.Out, provErrs)
	}
	return mixed, byProv, err
}

func cloneProviderCatalogMap(in map[string][]ListedModel) map[string][]ListedModel {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]ListedModel, len(in))
	for k, v := range in {
		out[k] = append([]ListedModel(nil), v...)
	}
	return out
}

func cachedProviderCatalogSlice(cfg *config.Root, prov string) ([]ListedModel, bool) {
	p := waitSlashCatalogPrefetch(context.Background(), cfg)
	if p == nil || p.byProvider == nil {
		return nil, false
	}
	cat, ok := p.byProvider[prov]
	if !ok || len(cat) == 0 {
		return nil, false
	}
	return append([]ListedModel(nil), cat...), true
}

func InvalidateAndPrefetchSlashModelCatalog(ctx context.Context, cfg *config.Root, out io.Writer) {
	InvalidateSlashModelCatalogCache()
	PrefetchSlashModelCatalog(ctx, cfg, out)
}
