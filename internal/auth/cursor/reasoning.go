package cursor

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelreasoning"

func captureVariantReasoning(flags []ModelFlag) {
	levels := map[string][]string{}
	defaults := map[string]string{}
	for _, flag := range flags {
		base := cursorBaseModelID(flag.ID)
		if base == "" {
			continue
		}
		if current := modelreasoning.Lookup("cursor-sub", flag.ID); current.Known && len(current.Levels) > 0 {
			levels[base] = append(levels[base], current.Levels...)
			if defaults[base] == "" {
				defaults[base] = current.Default
			}
		}
		level, _ := cursorVariantEffort(flag.ID, base)
		if len(modelreasoning.Normalize([]string{level})) > 0 {
			levels[base] = append(levels[base], level)
		}
		levels[base] = append(levels[base], variantChoiceEfforts(base)...)
	}
	for id, values := range levels {
		current := modelreasoning.Lookup("cursor-sub", id)
		if current.Known && len(current.Levels) == 0 {
			continue
		}
		merged := append(append([]string{}, current.Levels...), values...)
		if len(modelreasoning.Normalize(merged)) == 0 {
			continue
		}
		def := current.Default
		if def == "" {
			def = defaults[id]
		}
		modelreasoning.Store("cursor-sub", id, modelreasoning.Capability{Known: true, Levels: merged, Default: def})
	}
}
