package modelreasoning

import (
	"encoding/json"
	"strings"
	"sync"
)

type Capability struct {
	Known   bool     `json:"known"`
	Levels  []string `json:"levels"`
	Default string   `json:"default,omitempty"`
}

var order = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
var cache sync.Map

func Normalize(values []string) []string {
	result := []string{}
	for _, level := range order {
		for _, value := range values {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "extra-high" {
				value = "xhigh"
			}
			if value == "mid" {
				value = "medium"
			}
			if value == level {
				result = append(result, level)
				break
			}
		}
	}
	return result
}
func Classic() []string { return []string{"none", "low", "medium", "high", "xhigh", "max"} }
func (c Capability) Options() []string {
	if !c.Known {
		return Classic()
	}
	return append([]string{}, c.Levels...)
}
func (c Capability) Resolve(value string) string {
	options := c.Options()
	for _, level := range options {
		if level == value {
			return value
		}
	}
	for _, level := range options {
		if level == c.Default {
			return level
		}
	}
	for _, level := range options {
		if level == "medium" {
			return level
		}
	}
	if len(options) > 0 {
		return options[0]
	}
	return ""
}
func Store(origin, id string, c Capability) {
	c.Levels = Normalize(c.Levels)
	cache.Store(strings.TrimRight(origin, "/")+"\x00"+id, c)
}
func Lookup(origin, id string) Capability {
	if value, ok := cache.Load(strings.TrimRight(origin, "/") + "\x00" + id); ok {
		c := value.(Capability)
		c.Levels = append([]string{}, c.Levels...)
		return c
	}
	return Capability{Levels: []string{}}
}
func Capture(origin string, raw []byte) {
	var model map[string]json.RawMessage
	if json.Unmarshal(raw, &model) != nil {
		return
	}
	var id string
	for _, key := range []string{"id", "slug", "name", "serverModelName", "modelName"} {
		if json.Unmarshal(model[key], &id) == nil && id != "" {
			break
		}
	}
	if id != "" {
		Store(origin, id, Parse(raw))
	}
}
func CaptureList(origin string, raw []byte) {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return
	}
	for _, key := range []string{"data", "models", "items", "availableModels"} {
		var entries []json.RawMessage
		if json.Unmarshal(root[key], &entries) == nil {
			for _, entry := range entries {
				Capture(origin, entry)
			}
			return
		}
	}
}
func Parse(raw []byte) Capability {
	unknown := Capability{Levels: []string{}}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return unknown
	}
	if levels, ok := m["supported_reasoning_levels"]; ok && string(levels) != "null" {
		var entries []json.RawMessage
		if json.Unmarshal(levels, &entries) != nil {
			return unknown
		}
		values := []string{}
		for _, entry := range entries {
			var value string
			if json.Unmarshal(entry, &value) != nil {
				var p struct {
					Effort string `json:"effort"`
				}
				if json.Unmarshal(entry, &p) != nil {
					return unknown
				}
				value = p.Effort
			}
			values = append(values, value)
		}
		var def string
		_ = json.Unmarshal(m["default_reasoning_level"], &def)
		return capability(values, def)
	}
	if r := m["reasoning"]; len(r) > 0 && string(r) != "null" {
		var spec map[string]json.RawMessage
		if json.Unmarshal(r, &spec) == nil {
			if levels, ok := spec["supported_efforts"]; ok {
				values := []string{}
				if string(levels) == "null" {
					values = append(values, order...)
				} else if json.Unmarshal(levels, &values) != nil {
					return unknown
				}
				var mandatory bool
				_ = json.Unmarshal(spec["mandatory"], &mandatory)
				if mandatory {
					filtered := []string{}
					for _, v := range values {
						if v != "none" {
							filtered = append(filtered, v)
						}
					}
					values = filtered
				}
				var def string
				_ = json.Unmarshal(spec["default_effort"], &def)
				return capability(values, def)
			}
		}
	}
	if caps := m["capabilities"]; len(caps) > 0 {
		var all map[string]json.RawMessage
		_ = json.Unmarshal(caps, &all)
		if effort, ok := all["effort"]; ok {
			var spec map[string]json.RawMessage
			if json.Unmarshal(effort, &spec) != nil {
				return unknown
			}
			var enabled bool
			if json.Unmarshal(spec["supported"], &enabled) == nil && !enabled {
				return Capability{Known: true, Levels: []string{}}
			}
			values := []string{}
			found := false
			for _, level := range order {
				if value, ok := spec[level]; ok {
					var support struct {
						Supported bool `json:"supported"`
					}
					if json.Unmarshal(value, &support) == nil {
						found = true
						if support.Supported {
							values = append(values, level)
						}
					}
				}
			}
			if found {
				return capability(values, "")
			}
		}
	}
	var params []struct {
		ID              string            `json:"id"`
		Values          []json.RawMessage `json:"values"`
		SupportedValues []string          `json:"supportedValues"`
	}
	if json.Unmarshal(m["parameters"], &params) == nil {
		for _, p := range params {
			if p.ID == "reasoning" || p.ID == "effort" || p.ID == "reasoning_effort" || p.ID == "thinking" {
				values := append([]string{}, p.SupportedValues...)
				for _, raw := range p.Values {
					var value string
					if json.Unmarshal(raw, &value) != nil {
						var o struct {
							Value string `json:"value"`
						}
						if json.Unmarshal(raw, &o) != nil {
							return unknown
						}
						value = o.Value
					}
					values = append(values, value)
				}
				if p.Values != nil || p.SupportedValues != nil {
					return capability(values, "")
				}
			}
		}
	}
	var supported []string
	if raw, ok := m["supported_parameters"]; ok && string(raw) != "null" && json.Unmarshal(raw, &supported) == nil {
		hasReasoning := false
		for _, parameter := range supported {
			if parameter == "reasoning" || parameter == "reasoning_effort" {
				hasReasoning = true
			}
		}
		if !hasReasoning {
			return Capability{Known: true, Levels: []string{}}
		}
	}
	return unknown
}
func capability(values []string, def string) Capability {
	levels := Normalize(values)
	if len(values) > 0 && len(levels) == 0 {
		return Capability{Levels: []string{}}
	}
	return Capability{Known: true, Levels: levels, Default: def}
}
