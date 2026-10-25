package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflowAcceptsOnlyNewCalendarTags(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []struct{ ID, Run string } }
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, step := range workflow.Jobs["release"].Steps {
		if step.ID == "version" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("release version resolver missing")
	}
	for _, tc := range []struct{ tag, want string }{
		{"", "v2026.1024.4"}, {"v2026.1024.5", "v2026.1024.5"},
		{"v2026.1024.3", ""}, {"v2026.1023.5", ""}, {"v2026.1024.04", ""},
		{"v2026.1024.5; echo injected", ""}, {"v2026.1024.5-dev", ""},
	} {
		t.Run("requested="+tc.tag, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output")
			fixture := `
git() {
 case "$*" in
  'fetch --tags origin') return 0 ;;
  'tag -l '*) printf '%s\n' v2026.1024.0 v2026.1024.3 v2026.1024.not-a-revision ;;
  *) return 97 ;;
 esac
}
date() {
 case "$*" in '-u +%Y') echo 2026 ;; '-u +%m') echo 10 ;; '-u +%d') echo 24 ;; *) return 98 ;; esac
}
` + script
			command := exec.Command(installerBash(t), "-c", fixture)
			command.Env = append(os.Environ(), "REQUESTED_RELEASE_TAG="+tc.tag, "GITHUB_OUTPUT="+filepath.ToSlash(output))
			logs, err := command.CombinedOutput()
			if tc.want == "" {
				if err == nil {
					t.Fatalf("accepted invalid tag: %s", logs)
				}
				return
			}
			if err != nil {
				t.Fatalf("version selection: %v\n%s", err, logs)
			}
			selected, err := os.ReadFile(output)
			if err != nil || strings.TrimSpace(string(selected)) != "ver="+tc.want {
				t.Fatalf("version=%s, %v", selected, err)
			}
		})
	}
}
