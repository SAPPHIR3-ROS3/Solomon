package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands/connect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/providerui"
)

const modelCatalogTTL = time.Minute

type modelAPI struct {
	mu        sync.Mutex
	catalog   apiModelCatalog
	catalogAt time.Time
}

type apiModelChoice struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

type apiProviderCatalog struct {
	Complete         bool     `json:"complete"`
	Disabled         []string `json:"disabled,omitempty"`
	Models           []string `json:"models"`
	Provider         string   `json:"provider"`
	SupportsFastMode bool     `json:"supportsFastMode"`
}

type apiModelCatalog struct {
	Current   apiModelChoice       `json:"current"`
	Providers []apiProviderCatalog `json:"providers"`
	Recent    []apiModelChoice     `json:"recent"`
}

type apiModelVisibility struct {
	Enabled  bool   `json:"enabled"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

func newModelAPI() *modelAPI { return &modelAPI{} }

func (a *modelAPI) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	catalog, err := a.loadCatalog()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (a *modelAPI) handleCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.Provider = strings.TrimSpace(request.Provider)
	request.Model = strings.TrimSpace(request.Model)
	if request.Provider == "" || request.Model == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("provider and model are required"))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if config.ProviderByName(cfg, request.Provider) == nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("unknown provider %q", request.Provider))
		return
	}
	changed := cfg.Current.Provider != request.Provider || cfg.Current.Model != request.Model
	cfg.Current.Provider = request.Provider
	cfg.Current.Model = request.Model
	if changed {
		config.NoteRecentModelUse(cfg, request.Provider, request.Model)
	}
	if err := config.Save(cfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, apiModelChoice{Model: request.Model, Provider: request.Provider})
}

func (a *modelAPI) handleVisibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Enabled  *bool  `json:"enabled"`
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := decodeJSONBody(w, r, &request, 8192); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	request.Provider = strings.TrimSpace(request.Provider)
	request.Model = strings.TrimSpace(request.Model)
	if request.Enabled == nil || request.Provider == "" || request.Model == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("provider, model and enabled are required"))
		return
	}
	if err := config.QueueModelVisibility(request.Provider, request.Model, *request.Enabled); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusAccepted, apiModelVisibility{Enabled: *request.Enabled, Model: request.Model, Provider: request.Provider})
}

func (a *modelAPI) handleConnectProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		APIKey  string `json:"apiKey"`
		BaseURL string `json:"baseURL"`
		Kind    int    `json:"kind"`
		Name    string `json:"name"`
	}
	if err := decodeJSONBody(w, r, &request, 32<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	result, err := providerui.Connect(r.Context(), providerui.ConnectRequest{
		APIKey: request.APIKey, BaseURL: request.BaseURL, Kind: request.Kind, Name: request.Name,
	})
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.invalidate()
	writeJSON(w, http.StatusOK, apiModelChoice{Model: result.CurrentModel, Provider: result.CurrentProvider})
}

func (a *modelAPI) invalidate() {
	a.mu.Lock()
	a.catalogAt = time.Time{}
	a.mu.Unlock()
}

func (a *modelAPI) loadCatalog() (apiModelCatalog, error) {
	a.mu.Lock()
	if !a.catalogAt.IsZero() && time.Since(a.catalogAt) < modelCatalogTTL {
		catalog := a.catalog
		a.mu.Unlock()
		return catalog, nil
	}
	a.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return apiModelCatalog{}, err
	}
	catalog := apiModelCatalog{
		Current:   apiModelChoice{Model: strings.TrimSpace(cfg.Current.Model), Provider: strings.TrimSpace(cfg.Current.Provider)},
		Providers: []apiProviderCatalog{},
		Recent:    []apiModelChoice{},
	}
	for _, recent := range config.RecentModelUseEntries(cfg, cfg.Current.Provider) {
		if recent.Provider == config.ProviderNameClaudeSub {
			continue
		}
		catalog.Recent = append(catalog.Recent, apiModelChoice{Model: recent.Model, Provider: recent.Provider})
	}

	providers := config.ProviderList(cfg)
	filtered := make([]config.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider.Name != config.ProviderNameClaudeSub {
			filtered = append(filtered, provider)
		}
	}
	catalog.Providers = make([]apiProviderCatalog, len(filtered))
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(filtered))
	for index := range filtered {
		index := index
		provider := filtered[index]
		go func() {
			defer waitGroup.Done()
			ids, listErr := connect.ListModelsForProviderAll(ctx, cfg, &provider)
			ids = uniqueModelIDs(ids)
			liveCatalog := listErr == nil && len(ids) > 0
			if len(ids) == 0 {
				ids = uniqueModelIDs(cfg.RecentModels[provider.Name])
			}
			if provider.Name == cfg.Current.Provider {
				ids = ensureModelFirst(ids, cfg.Current.Model)
			}
			catalog.Providers[index] = apiProviderCatalog{
				Complete:         liveCatalog,
				Disabled:         config.HiddenModelIDs(cfg, provider.Name, ids),
				Models:           ids,
				Provider:         provider.Name,
				SupportsFastMode: config.FastModeSupportedByProvider(&provider),
			}
		}()
	}
	waitGroup.Wait()
	a.mu.Lock()
	a.catalog = catalog
	a.catalogAt = time.Now()
	a.mu.Unlock()
	return catalog, nil
}

func uniqueModelIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func ensureModelFirst(ids []string, current string) []string {
	current = strings.TrimSpace(current)
	if current == "" {
		return ids
	}
	result := []string{current}
	for _, id := range ids {
		if id != current {
			result = append(result, id)
		}
	}
	return result
}
