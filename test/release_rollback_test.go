package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseRollback(t *testing.T) {
	for _, mode := range []string{"present", "missing", "denied"} {
		t.Run(mode, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "calls")
			fixture := `
gh() {
 printf '%s\n' "$*" >> "$CALL_LOG"
 case "$*" in
  *releases/tags/*)
   if [[ "$MODE" == missing ]]; then printf 'HTTP/2.0 404 Not Found\n'; return 1; fi
   if [[ "$MODE" == denied ]]; then printf 'HTTP/2.0 403 Forbidden\n'; return 1; fi
   printf 'HTTP/2.0 200 OK\r\nContent-Type: application/json\r\n\r\n{"id":42}\n' ;;;
  *'--method DELETE'*) printf 'HTTP/2.0 404 Not Found\n'; return 1 ;;;
 esac
}
sleep() { :; }
source "$1"
`
			fixture = strings.ReplaceAll(fixture, ";;;", ";;")
			cmd := exec.Command(installerBash(t), "-c", fixture, "rollback-test", filepath.ToSlash(filepath.Join(repoRoot(t), "scripts/release/rollback.sh")))
			cmd.Env = append(os.Environ(), "RELEASE_TAG=v2026.1024.2", "PREVIOUS_TAG=v2026.1024.1", "GITHUB_REPOSITORY=owner/repo", "CALL_LOG="+filepath.ToSlash(log), "MODE="+mode)
			output, err := cmd.CombinedOutput()
			calls, readErr := os.ReadFile(log)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "denied" {
				if err == nil || strings.Contains(string(calls), "--method DELETE") {
					t.Fatalf("unsafe permission failure: %v %s", err, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("rollback: %v %s", err, output)
			}
			for _, want := range []string{"release edit v2026.1024.1 --repo owner/repo --latest", "--method DELETE repos/owner/repo/git/refs/tags/v2026.1024.2"} {
				if !strings.Contains(string(calls), want) {
					t.Fatalf("missing %q in %s", want, calls)
				}
			}
			if mode == "present" {
				for _, want := range []string{"--method PATCH repos/owner/repo/releases/42 -F draft=true", "--method DELETE repos/owner/repo/releases/42"} {
					if !strings.Contains(string(calls), want) {
						t.Fatalf("missing %q in %s", want, calls)
					}
				}
			}
		})
	}
}

func TestReleaseRollbackWorkflow(t *testing.T) {
	for _, path := range []string{".github/workflows/release.yml", ".github/actions/smoke/action.yml"} {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), path))
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), "\n") > 500 {
			t.Fatalf("%s exceeds 500 lines", path)
		}
	}
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cancel-in-progress: false", "scripts/release/rollback.sh", "inputs.smoke_release_tag == ''", "actions: write", "scripts/release/cleanup.sh", "previous_tag: ${{ steps.baseline.outputs.tag }}"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %q", want)
		}
	}
}
