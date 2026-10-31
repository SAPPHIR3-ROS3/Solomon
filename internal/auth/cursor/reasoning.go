package cursor

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/modelreasoning"

func captureVariantReasoning(flags []ModelFlag) {
	levels := map[string][]string{}
	for _, flag := range flags {
		base := cursorBaseModelID(flag.ID)
		if current := modelreasoning.Lookup("cursor-sub", flag.ID); current.Known {
			modelreasoning.Store("cursor-sub", base, current)
			continue
		}
		level, _ := cursorVariantEffort(flag.ID, base)
		if len(modelreasoning.Normalize([]string{level})) > 0 {
			levels[base] = append(levels[base], level)
		}
	}
	for id, values := range levels {
		if !modelreasoning.Lookup("cursor-sub", id).Known {
			modelreasoning.Store("cursor-sub", id, modelreasoning.Capability{Known: true, Levels: values})
		}
	}
}
