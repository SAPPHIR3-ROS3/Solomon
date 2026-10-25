package test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func skillSlashEntry(t *testing.T) skills.SkillEntry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: teach\ndescription: Teach.\n---\nTeach the user.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return skills.SkillEntry{Name: "teach", SkillMdPath: path}
}

func TestSkillSlashSendsTheTypedRequest(t *testing.T) {
	t.Parallel()
	var visible, api string
	d := commands.Deps{
		PrefillInput:             func(string) { t.Fatal("a typed request must be sent, not prefilled") },
		SubmitVisibleUserMessage: func(v, a string) error { visible, api = v, a; return nil },
	}
	line := "/teach Data Management, exam on the 28th"
	if err := commands.RunSkillSlash(d, skillSlashEntry(t), line); err != nil {
		t.Fatal(err)
	}
	if visible != line {
		t.Fatalf("visible=%q", visible)
	}
	if !strings.Contains(api, "Teach the user.") || !strings.Contains(api, "User request:\nData Management, exam on the 28th") {
		t.Fatalf("api message lost the skill or the request: %q", api)
	}
}

func TestSkillSlashWithoutRequestKeepsPrefill(t *testing.T) {
	t.Parallel()
	var prefilled string
	d := commands.Deps{
		PrefillInput:      func(s string) { prefilled = s },
		SubmitUserMessage: func(string) error { t.Fatal("bare skill command should prefill"); return nil },
	}
	if err := commands.RunSkillSlash(d, skillSlashEntry(t), "/teach  "); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prefilled, "Teach the user.") {
		t.Fatalf("prefilled=%q", prefilled)
	}
}

func TestOrchestrateGoDirectiveVersion(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		"go1.26.10\n":         "1.26.10",
		"go1.26.9-X:nodwarf5": "1.26.9",
		"go1.27rc1":           "1.27",
		"devel +abc":          "",
		"":                    "",
	} {
		if got := compile.GoDirectiveVersion(raw); got != want {
			t.Errorf("GoDirectiveVersion(%q)=%q, want %q", raw, got, want)
		}
	}
}

func TestOrchestrateRequiredGoVersionMatchesGoMod(t *testing.T) {
	t.Parallel()
	goMod, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(goMod), "\n") {
		if version, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			if version != compile.RequiredGoVersion {
				t.Fatalf("go.mod requires go %s, orchestrate requires %s", version, compile.RequiredGoVersion)
			}
			return
		}
	}
	t.Fatal("go.mod has no go directive")
}

func TestOrchestrateCheckGoToolchain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version, toolchain string
		ok                 bool
	}{
		{"go" + compile.RequiredGoVersion, "local", true},
		{"go1.28.0", "local", true},
		{"devel go1.28-abc", "local", true},
		{"go1.26.10", "auto", true},
		{"go1.26.10", "go1.26.10+auto", true},
		{"go1.26.10", "path", true},
		{"go1.26.10", "local", false},
		{"go1.27rc1", "local", false},
		{"go1.26.10", "go1.26.10", false},
	} {
		err := compile.CheckGoToolchain(tc.version, tc.toolchain)
		if (err == nil) != tc.ok {
			t.Errorf("CheckGoToolchain(%q, %q) = %v, want ok=%v", tc.version, tc.toolchain, err, tc.ok)
		}
		if err != nil && (!strings.Contains(err.Error(), compile.RequiredGoVersion) || !strings.Contains(err.Error(), "GOTOOLCHAIN=auto")) {
			t.Errorf("error is not actionable: %v", err)
		}
	}
}

// An old local go that may not switch toolchains must fail with the fix, before any build runs.
func TestOrchestrateBuildRejectsIncompatibleLocalToolchain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake go command is a shell script")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \"$1 $2\" in\n\"env GOVERSION\") echo go1.26.10 ;;\n\"env GOTOOLCHAIN\") echo local ;;\n*) echo \"unexpected go $*\" >&2; exit 1 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := compile.BuildWASM(compile.Options{Source: "package main\n\nfunc main() {}\n", ModuleRoot: t.TempDir()})
	if err == nil {
		t.Fatal("build succeeded with an incompatible toolchain")
	}
	for _, want := range []string{"requires Go " + compile.RequiredGoVersion, "go1.26.10", "GOTOOLCHAIN=local", "GOTOOLCHAIN=auto"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}
