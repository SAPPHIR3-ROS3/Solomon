package cursor

import (
	"sort"
	"strings"
)

const DefaultModelID = "composer-2.5"

func DefaultModelIDs() []string {
	return []string{DefaultModelID, "auto"}
}

type cursorLab int

const (
	labOpenAI cursorLab = iota
	labAnthropic
	labXAI
	labGoogle
	labComposer
	labAuto
	labGLM
	labKimi
	labOther
)

func FilterModelIDs(ids []string) []string {
	if len(ids) == 0 {
		return DefaultModelIDs()
	}
	byLab := make(map[cursorLab][]string)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		lab := classifyCursorLab(id)
		if lab == labOther {
			continue
		}
		byLab[lab] = append(byLab[lab], id)
	}
	order := []cursorLab{labOpenAI, labAnthropic, labXAI, labGoogle, labComposer, labAuto, labGLM, labKimi}
	var out []string
	for _, lab := range order {
		if id := pickLabFlagship(lab, byLab[lab]); id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return DefaultModelIDs()
	}
	return out
}

func OrderModelIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	byLab := make(map[cursorLab][]string)
	var other []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		lab := classifyCursorLab(id)
		if lab == labOther {
			other = append(other, id)
			continue
		}
		byLab[lab] = append(byLab[lab], id)
	}
	claimed := map[string]bool{}
	var out []string
	appendIDs := func(ids []string) {
		for _, id := range ids {
			if claimed[id] {
				continue
			}
			out = append(out, id)
			claimed[id] = true
		}
	}
	priorityLabs := []cursorLab{labOpenAI, labAnthropic, labXAI, labGoogle, labComposer}
	for _, lab := range priorityLabs {
		if id := pickLabFlagship(lab, byLab[lab]); id != "" {
			appendIDs([]string{id})
		}
	}
	appendIDs(byLab[labAuto])
	if id := pickLabFlagship(labGLM, byLab[labGLM]); id != "" {
		appendIDs([]string{id})
	}
	if id := pickLabFlagship(labKimi, byLab[labKimi]); id != "" {
		appendIDs([]string{id})
	}
	for _, lab := range []cursorLab{labOpenAI, labAnthropic, labXAI, labGoogle, labComposer, labGLM, labKimi} {
		for _, id := range byLab[lab] {
			if !claimed[id] {
				other = append(other, id)
			}
		}
	}
	sort.Strings(other)
	return append(out, other...)
}

func sortLabModelIDs(lab cursorLab, ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	type item struct {
		id string
		sc flagshipScore
	}
	items := make([]item, 0, len(ids))
	for _, id := range ids {
		sc := scoreLabModel(lab, id)
		items = append(items, item{id: id, sc: sc})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].sc.ok != items[j].sc.ok {
			return items[i].sc.ok
		}
		if !items[i].sc.ok {
			return items[i].id < items[j].id
		}
		return flagshipBetter(items[i].sc, items[j].sc)
	})
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].id
	}
	return out
}

func classifyCursorLab(id string) cursorLab {
	m := strings.ToLower(strings.TrimSpace(id))
	switch {
	case m == "auto":
		return labAuto
	case strings.HasPrefix(m, "composer"):
		return labComposer
	case strings.HasPrefix(m, "gpt") || strings.Contains(m, "openai"):
		return labOpenAI
	case strings.Contains(m, "claude"):
		return labAnthropic
	case strings.Contains(m, "grok"):
		return labXAI
	case strings.Contains(m, "gemini") || strings.Contains(m, "google"):
		return labGoogle
	case strings.Contains(m, "glm"):
		return labGLM
	case strings.Contains(m, "kimi"):
		return labKimi
	default:
		return labOther
	}
}

func pickLabFlagship(lab cursorLab, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	if lab == labAuto {
		for _, id := range ids {
			if strings.EqualFold(id, "auto") {
				return id
			}
		}
		return ""
	}
	var best string
	var bestSc flagshipScore
	for _, id := range ids {
		sc := scoreLabModel(lab, id)
		if !sc.ok {
			continue
		}
		if best == "" || flagshipBetter(sc, bestSc) {
			best = id
			bestSc = sc
		}
	}
	return best
}

type flagshipScore struct {
	ver      []int
	lineTier int
	tier     int
	ok       bool
}
