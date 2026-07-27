//go:build automatic_role_scores

package benchmarks

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const defaultMaxModels = 48

type BuildOptions struct {
	APIKey    string
	Commit    string
	MaxModels int
}

func BuildScores(ctx context.Context, opts BuildOptions) (ScoresFile, Manifest, error) {
	_ = ctx
	_ = opts
	return ScoresFile{}, Manifest{}, fmt.Errorf("automatic benchmark score calculation is disabled: assign role scores manually")
	/*
		if ctx == nil {
			ctx = context.Background()
		}
		maxModels := opts.MaxModels
		if maxModels <= 0 {
			maxModels = defaultMaxModels
		}
		models, tier, err := fetchAAModels(ctx, opts.APIKey)
		if err != nil {
			return ScoresFile{}, Manifest{}, err
		}
		selected := selectPriorityModels(models, maxModels)
		if len(selected) == 0 {
			return ScoresFile{}, Manifest{}, fmt.Errorf("no models selected after priority filter")
		}
		keys := make([]string, 0, len(selected))
		byKey := map[string]aaModel{}
		for _, m := range selected {
			key := m.canonicalKey()
			if key == "" {
				continue
			}
			if prev, ok := byKey[key]; !ok || m.priorityScore() > prev.priorityScore() {
				byKey[key] = m
			}
		}
		for key := range byKey {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return roles.CompareModelRecency(keys[i], keys[j]) > 0
		})
		raw := buildRawCharacteristicValues(keys, byKey)
		outModels := map[string]map[string]int{}
		for _, ch := range allCharacteristics() {
			if ch == "consistency" {
				continue
			}
			perSource, ok := characteristicSources[ch]
			if !ok || len(perSource) == 0 {
				continue
			}
			sourceScores := make([]map[string]int, len(perSource))
			for si, src := range perSource {
				var presentKeys []string
				var vals []float64
				for _, key := range keys {
					rawValue := raw[key][sourceKey(ch, si)]
					if !rawValue.present {
						continue
					}
					presentKeys = append(presentKeys, key)
					vals = append(vals, rawValue.value)
				}
				sourceScores[si] = map[string]int{}
				if len(vals) == 0 {
					continue
				}
				norm := scoring.GapAwareNormalize(vals, src.higher)
				for i, key := range presentKeys {
					sourceScores[si][key] = scoring.RoundScore(norm[i])
				}
			}
			for _, key := range keys {
				var parts []float64
				for si := range perSource {
					if v, ok := sourceScores[si][key]; ok && raw[key][sourceKey(ch, si)].present {
						parts = append(parts, float64(v))
					}
				}
				if len(parts) == 0 {
					continue
				}
				if outModels[key] == nil {
					outModels[key] = map[string]int{}
				}
				if len(parts) == 1 {
					outModels[key][ch] = scoring.RoundScore(parts[0])
					continue
				}
				outModels[key][ch] = scoring.RoundScore(scoring.AggregateF2(parts))
			}
		}
		for _, key := range keys {
			var rawVals []float64
			for _, v := range byKey[key].Evals {
				rawVals = append(rawVals, v)
			}
			if len(rawVals) == 0 {
				continue
			}
			if outModels[key] == nil {
				outModels[key] = map[string]int{}
			}
			outModels[key]["consistency"] = scoring.ConsistencyFromSources(rawVals)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		sourceLabel := "artificial_analysis"
		if tier != "" {
			sourceLabel = "artificial_analysis:" + tier
		}
		scores := ScoresFile{
			Version:     2,
			GeneratedAt: now,
			Sources:     []string{sourceLabel},
			Models:      outModels,
		}
		manifest := Manifest{
			GeneratedAt: now,
			Commit:      strings.TrimSpace(opts.Commit),
		}
		return scores, manifest, nil
	*/
}

func sourceKey(ch string, idx int) string {
	return fmt.Sprintf("%s:%d", ch, idx)
}

type rawCharacteristicValue struct {
	value   float64
	present bool
}

func buildRawCharacteristicValues(keys []string, byKey map[string]aaModel) map[string]map[string]rawCharacteristicValue {
	out := map[string]map[string]rawCharacteristicValue{}
	for _, key := range keys {
		m := byKey[key]
		out[key] = map[string]rawCharacteristicValue{}
		for ch, specs := range characteristicSources {
			for si, spec := range specs {
				value, present := rawValueForSpec(m, spec)
				out[key][sourceKey(ch, si)] = rawCharacteristicValue{value: value, present: present}
			}
			_ = ch
		}
	}
	return out
}

func rawValueForSpec(m aaModel, spec sourceField) (float64, bool) {
	for _, key := range spec.keys {
		if v, ok := m.Evals[key]; ok {
			return v, true
		}
	}
	switch spec.keys[0] {
	case "list_price_usd_per_1m_tokens":
		if m.ListPrice > 0 {
			return m.ListPrice, true
		}
	case "intelligence_index_total_cost_usd":
		if m.IntelCost > 0 {
			return m.IntelCost, true
		}
	case "coding_agent_total_cost_usd":
		if m.CodingCost > 0 {
			return m.CodingCost, true
		}
		if m.IntelCost > 0 {
			return m.IntelCost, true
		}
	case "median_output_tokens_per_second":
		if m.TokPerSec > 0 {
			return m.TokPerSec, true
		}
	}
	return 0, false
}

func selectPriorityModels(models []aaModel, max int) []aaModel {
	priority := map[string]bool{}
	for _, p := range []string{
		"gpt-5-6", "gpt-5-4", "claude-opus-4-8", "claude-sonnet-4-6",
		"gemini-2-5-pro", "kimi-k2-7", "grok-4-5", "glm-5-2",
		"composer-2-5", "deepseek-r1", "qwen3",
	} {
		priority[p] = true
	}
	filtered := make([]aaModel, 0, len(models))
	for _, m := range models {
		if m.Deprecated {
			continue
		}
		if m.Intelligence <= 0 && m.CodingIndex <= 0 && m.MathIndex <= 0 {
			if !matchesPriorityFamily(m.Slug, priority) {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].priorityScore() > filtered[j].priorityScore()
	})
	if len(filtered) <= max {
		return filtered
	}
	out := append([]aaModel(nil), filtered[:max]...)
	seen := map[string]bool{}
	for _, m := range out {
		seen[m.canonicalKey()] = true
	}
	for _, m := range filtered[max:] {
		if matchesPriorityFamily(m.Slug, priority) {
			key := m.canonicalKey()
			if key != "" && !seen[key] {
				out = append(out, m)
				seen[key] = true
			}
		}
	}
	return out
}

func matchesPriorityFamily(slug string, priority map[string]bool) bool {
	s := strings.ToLower(strings.ReplaceAll(slug, "_", "-"))
	for p := range priority {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
