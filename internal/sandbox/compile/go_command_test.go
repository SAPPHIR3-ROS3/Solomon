package compile

import (
	"strings"
	"testing"
)

func TestGoCommandSelectsProjectToolchain(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "go1.27.0")
	t.Setenv("GOWORK", "/unrelated/go.work")
	cmd := orchestrateGoCommand("go", "version")
	for key, want := range map[string]string{
		"GOTOOLCHAIN": "go" + RequiredGoVersion(),
		"GOWORK":      "off",
	} {
		var values []string
		for _, entry := range cmd.Env {
			name, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(name, key) {
				values = append(values, value)
			}
		}
		if len(values) != 1 || values[0] != want {
			t.Fatalf("%s = %q, want exactly %q", key, values, want)
		}
	}
}
