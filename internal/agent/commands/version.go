package commands

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

var version = "dev"
var commit = ""

func VersionString() string {
	if v := strings.TrimSpace(version); v != "" && v != "dev" {
		return v
	}
	base := strings.TrimSpace(version)
	if base == "" {
		base = "dev"
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := strings.TrimSpace(info.Main.Version); v != "" && v != "(devel)" && !isGoPseudoVersion(v) {
		return v
	}
	var revision string
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		revision = strings.TrimSpace(commit)
	}
	if revision == "" {
		return base
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if base == "dev" {
		if modified {
			return fmt.Sprintf("dev-%s-dirty", revision)
		}
		return fmt.Sprintf("dev-%s", revision)
	}
	if modified {
		return fmt.Sprintf("%s-dev-%s-dirty", base, revision)
	}
	return fmt.Sprintf("%s-dev-%s", base, revision)
}

func isGoPseudoVersion(v string) bool {
	if strings.HasPrefix(v, "v0.0.0-") {
		return true
	}
	dash := strings.IndexByte(v, '-')
	if dash <= 0 || !strings.HasPrefix(v[dash+1:], "0.") {
		return false
	}
	return strings.Contains(v[dash+1:], "-")
}

func WriteVersion(w io.Writer) {
	termcolor.WriteSystem(w, VersionString())
}

func Version(d Deps) error {
	WriteVersion(d.Out)
	return nil
}
