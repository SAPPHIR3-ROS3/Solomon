// Package solomon exposes build metadata shared by the application and Code Mode.
package solomon

import (
	_ "embed"
	"strings"
)

// Embed the actual module directive so installed binaries do not need the source
// checkout and Go upgrades cannot leave Code Mode on a separate version.
//
//go:embed go.mod
var moduleDefinition string

// RequiredGoVersion returns the minimum Go version declared by this build.
func RequiredGoVersion() string {
	for _, line := range strings.Split(moduleDefinition, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	panic("embedded Solomon go.mod has no go directive")
}
