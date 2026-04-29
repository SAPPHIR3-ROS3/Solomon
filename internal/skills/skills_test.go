package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeRepoURL(t *testing.T) {
	got, err := NormalizeRepoURL("o/r")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://github.com/o/r"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got, err = NormalizeRepoURL("https://GITHUB.com/O/R.git")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://github.com/O/R"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStableKeyHexStable(t *testing.T) {
	k1 := StableKeyHex("https://github.com/a/b", ".")
	k2 := StableKeyHex("https://github.com/a/b", ".")
	if k1 != k2 || len(k1) != 64 {
		t.Fatalf("key %q len %d", k1, len(k1))
	}
}

func TestParseAddArgs(t *testing.T) {
	p, err := ParseAddArgs([]string{"skill", "x/y", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Scope != ScopeLocal || p.GitHubRef != "x/y" {
		t.Fatalf("%+v", p)
	}
	p, err = ParseAddArgs([]string{"https://skills.sh/foo/bar", "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromSkillsSh || p.Scope != ScopeGlobal {
		t.Fatalf("%+v", p)
	}
}

func TestParseSkillFrontMatter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	content := "---\nname: MySkill\n---\nbody\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	fm, err := ParseSkillFrontMatter(p)
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "MySkill" {
		t.Fatalf("%v", fm)
	}
}

func TestRepoOwner(t *testing.T) {
	if o := RepoOwner("https://github.com/acme/cool"); o != "acme" {
		t.Fatalf("got %q", o)
	}
}

func TestUniqueDisplayName(t *testing.T) {
	r := NewRegistry()
	k1 := StableKeyHex("https://github.com/a/x", ".")
	k2 := StableKeyHex("https://github.com/b/y", ".")
	r.Global[k1] = SkillEntry{Name: "Docs"}
	got := UniqueDisplayName(r, "https://github.com/b/y", "Docs", ScopeGlobal, "", k2)
	if want := "b-Docs"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	r.Global[k2] = SkillEntry{Name: "b-Docs"}
	k3 := StableKeyHex("https://github.com/c/z", ".")
	got2 := UniqueDisplayName(r, "https://github.com/c/z", "Docs", ScopeGlobal, "", k3)
	if got2 != "c-Docs" {
		t.Fatalf("got %q want c-Docs", got2)
	}
	r.Global[k3] = SkillEntry{Name: "c-Docs"}
	k4 := StableKeyHex("https://github.com/c/w", ".")
	got3 := UniqueDisplayName(r, "https://github.com/c/w", "Docs", ScopeGlobal, "", k4)
	if got3 != "c-Docs-2" {
		t.Fatalf("got %q want c-Docs-2", got3)
	}
}

func TestParseRemoveArgs(t *testing.T) {
	_, err := ParseRemoveArgs([]string{"skill"})
	if err == nil {
		t.Fatal("want err")
	}
	n, err := ParseRemoveArgs([]string{"skill", "My", "Skill"})
	if err != nil || n != "My Skill" {
		t.Fatalf("%q %v", n, err)
	}
}

func TestRunRemove_global(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	uniq := filepath.Base(home)
	skillName := "RmSkill_" + uniq
	skillDir := filepath.Join(home, ".solomon", "skills", "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdab")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	k := "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdab"
	reg.Global[k] = SkillEntry{Name: skillName, ClonePath: skillDir}
	b, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, b, 0o600); err != nil {
		t.Fatal(err)
	}
	projRoot := filepath.Join(home, "workspace")
	if err := os.MkdirAll(projRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	opts := RemoveOpts{ProjHex: "00", ProjRoot: projRoot, Args: []string{"skill", skillName}}
	if err := RunRemove(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatal("clone dir should be gone")
	}
	b2, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Registry
	if err := json.Unmarshal(b2, &r2); err != nil {
		t.Fatal(err)
	}
	if len(r2.Global) != 0 {
		t.Fatalf("global map: %v", r2.Global)
	}
}
