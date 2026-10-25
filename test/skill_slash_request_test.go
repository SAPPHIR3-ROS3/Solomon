package test

import (
	"os"
	"path/filepath"
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

func TestOrchestrateStubGoDirectiveVersion(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		"go1.26.10\n":         "1.26.10",
		"go1.26.9-X:nodwarf5": "1.26.9",
		"go1.27rc1":           "1.27",
		"devel +abc":          "1.25.0",
		"":                    "1.25.0",
	} {
		if got := compile.GoDirectiveVersion(raw); got != want {
			t.Errorf("GoDirectiveVersion(%q)=%q, want %q", raw, got, want)
		}
	}
}
