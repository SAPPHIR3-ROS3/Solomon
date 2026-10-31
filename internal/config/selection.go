package config

import (
	"errors"
	"net/url"
	"sort"
	"strings"
)

func NoteRecentModelUse(r *Root, providerName, modelID string) {
	if r == nil {
		return
	}
	prov := strings.TrimSpace(providerName)
	mid := strings.TrimSpace(modelID)
	if prov == "" || mid == "" {
		return
	}
	if r.RecentModels == nil {
		r.RecentModels = make(map[string][]string)
	}
	list := r.RecentModels[prov]
	out := []string{mid}
	for _, m := range list {
		if strings.TrimSpace(m) == mid {
			continue
		}
		out = append(out, m)
		if len(out) >= recentModelUseCap {
			break
		}
	}
	r.RecentModels[prov] = out
}

// ModelEnabled reports whether a model should be offered by the GUI model selector.
// Models are enabled by default; only explicitly hidden models are stored.
func ModelEnabled(r *Root, providerName, modelID string) bool {
	if r == nil {
		return true
	}
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return true
	}
	for _, hidden := range r.HiddenModels[providerName] {
		if strings.TrimSpace(hidden) == modelID {
			return false
		}
	}
	return true
}

// SetModelEnabled changes the GUI visibility preference for one model.
func SetModelEnabled(r *Root, providerName, modelID string, enabled bool) error {
	if r == nil {
		return errors.New("config is required")
	}
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" {
		return errors.New("provider and model are required")
	}
	if r.HiddenModels == nil {
		r.HiddenModels = make(map[string][]string)
	}
	current := r.HiddenModels[providerName]
	if enabled {
		out := make([]string, 0, len(current))
		for _, hidden := range current {
			if strings.TrimSpace(hidden) == "" || strings.TrimSpace(hidden) == modelID {
				continue
			}
			out = append(out, hidden)
		}
		if len(out) == 0 {
			delete(r.HiddenModels, providerName)
		} else {
			r.HiddenModels[providerName] = out
		}
		return nil
	}
	for _, hidden := range current {
		if strings.TrimSpace(hidden) == modelID {
			return nil
		}
	}
	r.HiddenModels[providerName] = append(current, modelID)
	return nil
}

// HiddenModelIDs returns the hidden models for a provider that are present in
// the supplied catalog. The catalog remains complete so settings can re-enable
// every model later.
func HiddenModelIDs(r *Root, providerName string, models []string) []string {
	if r == nil || len(models) == 0 {
		return nil
	}
	hidden := make(map[string]struct{}, len(r.HiddenModels[providerName]))
	for _, model := range r.HiddenModels[strings.TrimSpace(providerName)] {
		hidden[strings.TrimSpace(model)] = struct{}{}
	}
	if len(hidden) == 0 {
		return nil
	}
	out := make([]string, 0)
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := hidden[model]; !ok {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func RecentModelUseEntries(r *Root, preferProvider string) []RecentModelUse {
	if r == nil || len(r.RecentModels) == 0 {
		return nil
	}
	prefer := strings.TrimSpace(preferProvider)
	names := make([]string, 0, len(r.RecentModels))
	for name := range r.RecentModels {
		names = append(names, name)
	}
	sort.Strings(names)
	var ordered []string
	if prefer != "" {
		ordered = append(ordered, prefer)
		for _, name := range names {
			if name != prefer {
				ordered = append(ordered, name)
			}
		}
	} else {
		ordered = names
	}
	var out []RecentModelUse
	for _, prov := range ordered {
		for _, model := range r.RecentModels[prov] {
			mid := strings.TrimSpace(model)
			if mid == "" {
				continue
			}
			out = append(out, RecentModelUse{Provider: prov, Model: mid})
		}
	}
	return out
}

func ProviderByName(r *Root, name string) *Provider {
	if r == nil || len(r.Providers) == 0 {
		return nil
	}
	return r.Providers[name]
}

func ProviderList(r *Root) []Provider {
	if r == nil || len(r.Providers) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.Providers))
	for name := range r.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Provider, len(names))
	for i, name := range names {
		p := r.Providers[name]
		if p == nil {
			continue
		}
		cp := *p
		cp.Name = name
		out[i] = cp
	}
	return out
}

func FirstProviderName(r *Root) string {
	list := ProviderList(r)
	if len(list) == 0 {
		return ""
	}
	return list[0].Name
}

func IsLocalEndpoint(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return false
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0:0:0:0:0:0:0:1":
		return true
	}
	return strings.HasSuffix(host, ".local")
}

func RemoteProviderNames(r *Root) []string {
	list := ProviderList(r)
	if len(list) == 0 {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, p := range list {
		if !IsLocalEndpoint(p.BaseURL) {
			names = append(names, p.Name)
		}
	}
	return names
}

func (r *Root) WebSearchNeedsInternet() bool {
	if r == nil {
		return true
	}
	engine := strings.ToLower(strings.TrimSpace(r.EffectiveWebSearchEngine()))
	if engine == "searxng" || engine == "searx" {
		if IsLocalEndpoint(r.WebSearchBaseURL) {
			return false
		}
	}
	return true
}
