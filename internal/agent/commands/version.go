package commands

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

var version = "dev"
var commit = ""
var commitTime = ""

var effectiveReleaseVersion struct {
	sync.RWMutex
	tag                 string
	matchesReleaseAsset bool
}

// BuildCommit returns the full source revision embedded in the build.
func BuildCommit() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && strings.TrimSpace(setting.Value) != "" {
				return strings.TrimSpace(setting.Value)
			}
		}
	}
	return strings.TrimSpace(commit)
}

// BuildCommitTime returns the commit timestamp embedded in the build.
func BuildCommitTime() time.Time {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key != "vcs.time" {
				continue
			}
			if timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(setting.Value)); err == nil {
				return timestamp
			}
		}
	}
	if timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(commitTime)); err == nil {
		return timestamp
	}
	return time.Time{}
}

// SetEffectiveReleaseVersion records the latest published version used as the
// base for a development build's display version. Only a verified match of both
// the source commit and the published asset digest displays the release tag.
func SetEffectiveReleaseVersion(tag string, matchesReleaseAsset bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return
	}
	effectiveReleaseVersion.Lock()
	effectiveReleaseVersion.tag = tag
	effectiveReleaseVersion.matchesReleaseAsset = matchesReleaseAsset
	effectiveReleaseVersion.Unlock()
}

func VersionString() string {
	effectiveReleaseVersion.RLock()
	tag, matchesReleaseAsset := effectiveReleaseVersion.tag, effectiveReleaseVersion.matchesReleaseAsset
	effectiveReleaseVersion.RUnlock()
	if matchesReleaseAsset && tag != "" {
		return tag
	}
	raw := strings.TrimSpace(version)
	if raw == "" {
		raw = "dev"
	}
	base, development := developmentBase(raw)
	if !development {
		return raw
	}
	if tag != "" {
		base = tag
	}
	info, _ := debug.ReadBuildInfo()
	if base == "dev" && tag == "" && info != nil {
		if v := strings.TrimSpace(info.Main.Version); v != "" && v != "(devel)" && !isGoPseudoVersion(v) {
			return v
		}
	}
	var modified bool
	if info != nil {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.modified" {
				modified = setting.Value == "true"
			}
		}
	}
	revision := BuildCommit()
	if revision == "" {
		if base == "dev" {
			return "dev"
		}
		return base + "-dev"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified {
		return fmt.Sprintf("%s-dev-%s-dirty", base, revision)
	}
	return fmt.Sprintf("%s-dev-%s", base, revision)
}

func developmentBase(raw string) (string, bool) {
	if raw == "dev" {
		return "dev", true
	}
	if strings.HasPrefix(raw, "dev-") {
		return "dev", true
	}
	if marker := strings.Index(raw, "-dev-"); marker > 0 {
		return raw[:marker], true
	}
	if strings.HasSuffix(raw, "-dev") {
		base := strings.TrimSuffix(raw, "-dev")
		if base == "" {
			base = "dev"
		}
		return base, true
	}
	return raw, false
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
