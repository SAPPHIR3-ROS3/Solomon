package config

import (
	toml "github.com/pelletier/go-toml/v2"
	"testing"
)

func TestFileChangesDefaultAndExplicitPreference(t *testing.T) {
	for _, tc := range []struct {
		source  string
		enabled bool
	}{{"", true}, {"show_file_changes = true", true}, {"show_file_changes = false", false}} {
		t.Run(tc.source, func(t *testing.T) {
			var file rootFile
			if err := toml.Unmarshal([]byte(tc.source), &file); err != nil {
				t.Fatal(err)
			}
			root := rootFromFile(&file)
			if root.FileChangesEnabled() != tc.enabled {
				t.Fatalf("enabled=%t, want %t", root.FileChangesEnabled(), tc.enabled)
			}
			encoded, err := toml.Marshal(rootToFile(root))
			if err != nil {
				t.Fatal(err)
			}
			var persisted rootFile
			if err := toml.Unmarshal(encoded, &persisted); err != nil {
				t.Fatal(err)
			}
			if rootFromFile(&persisted).FileChangesEnabled() != tc.enabled {
				t.Fatal("roundtrip changed preference")
			}
			var legacy rootLegacyFile
			if err := toml.Unmarshal([]byte(tc.source), &legacy); err != nil {
				t.Fatal(err)
			}
			if rootFromLegacy(&legacy).FileChangesEnabled() != tc.enabled {
				t.Fatal("legacy preference differs")
			}
		})
	}
	if !(&Root{}).FileChangesEnabled() {
		t.Fatal("empty configuration must enable summaries")
	}
}
