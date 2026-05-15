package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

func TestNormalizeRepoURL(t *testing.T) {
	got, err := skills.NormalizeRepoURL("o/r")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://github.com/o/r"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got, err = skills.NormalizeRepoURL("https://GITHUB.com/O/R.git")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://github.com/O/R"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStableKeyHexStable(t *testing.T) {
	k1 := skills.StableKeyHex("https://github.com/a/b", ".")
	k2 := skills.StableKeyHex("https://github.com/a/b", ".")
	if k1 != k2 || len(k1) != 64 {
		t.Fatalf("key %q len %d", k1, len(k1))
	}
}

func TestParseAddArgs(t *testing.T) {
	p, err := skills.ParseAddArgs([]string{"npx", "--yes", "skills", "add", "a/b", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Scope != skills.ScopeLocal || !strings.Contains(p.NpmCommand, "skills") {
		t.Fatalf("%+v", p)
	}
	p, err = skills.ParseAddArgs([]string{"https://skills.sh/foo/bar", "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromSkillsSh || p.Scope != skills.ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	p, err = skills.ParseAddArgs([]string{"skill", "https://example.com/r/skill.md", "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.RemoteMDURL != "https://example.com/r/skill.md" || p.Scope != skills.ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	p, err = skills.ParseAddArgs([]string{"skill", "https://example.com/r/skill.md", "Disp", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.DisplayName != "Disp" || p.Scope != skills.ScopeProject {
		t.Fatalf("%+v", p)
	}
	if _, err := skills.ParseAddArgs([]string{"skill", "https://example.com/r"}); err == nil {
		t.Fatal("want err for non-.md URL")
	}
	dir := t.TempDir()
	localMD := filepath.Join(dir, "loc.md")
	if err := os.WriteFile(localMD, []byte("---\nname: L\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err = skills.ParseAddArgs([]string{"skill", localMD, "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.RemoteMDURL != localMD || p.Scope != skills.ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	if _, err := skills.ParseAddArgs([]string{"skill", "ftp://a/b.md", "global"}); err == nil {
		t.Fatal("want err for unsupported URL scheme")
	}
}

func TestIsSkillMarkdownSource(t *testing.T) {
	if !skills.IsSkillMarkdownSource("https://a/b.md") || !skills.IsSkillMarkdownSource("/tmp/x.md") || !skills.IsSkillMarkdownSource("file:///tmp/x.md") {
		t.Fatal("expected accepted sources")
	}
	if skills.IsSkillMarkdownSource("ftp://a/b.md") || skills.IsSkillMarkdownSource("https://a/x") || skills.IsSkillMarkdownSource("npx foo") {
		t.Fatal("expected rejected sources")
	}
}

func TestNormalizeSkillMarkdownSource_local(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := skills.NormalizeSkillMarkdownSource(p)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := skills.NormalizeSkillMarkdownSource(filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("want err for missing file")
	}
}

func TestInstallShellCommandMeta(t *testing.T) {
	m := &skills.SkillsShMeta{RepoURL: "https://github.com/gh/awesome-copilot", PreferredSkill: "prd"}
	got := m.InstallShellCommand()
	if !strings.Contains(got, "github.com/gh/awesome-copilot") || !strings.Contains(got, "--skill prd") ||
		!strings.Contains(got, "-g") || !strings.Contains(got, "-y") {
		t.Fatalf("%q", got)
	}
}

func TestEnsureSkillsAddGlobalYes(t *testing.T) {
	raw := "npx skills add https://github.com/a/b --skill prd"
	got := skills.EnsureSkillsAddGlobalYes(raw)
	if !strings.Contains(got, "-g") || !strings.Contains(got, "-y") {
		t.Fatalf("%q", got)
	}
	complete := "npx --yes skills add -g -y https://github.com/a/b --skill prd"
	if skills.EnsureSkillsAddGlobalYes(complete) != complete {
		t.Fatalf("should not alter already-flagged command")
	}
}

func TestPartitionInstalledSkills_localAndProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projHex := "00"
	projRoot := filepath.Join(home, "workspace")
	prSkills, err := paths.ProjectSkillsDir(projHex)
	if err != nil {
		t.Fatal(err)
	}
	localClone := filepath.Join(paths.LocalSkillsDir(projRoot), "h1")
	projClone := filepath.Join(prSkills, "h2")
	r := skills.NewRegistry()
	r.Projects[projHex] = map[string]skills.SkillEntry{
		"a": {Name: "Loc", ClonePath: localClone, SourceRepo: "u1"},
		"b": {Name: "Prj", ClonePath: projClone, SourceRepo: "u2"},
	}
	l, p, g := skills.PartitionInstalledSkills(r, projHex, projRoot)
	if len(l) != 1 || len(p) != 1 || len(g) != 0 || l[0].Name != "Loc" || p[0].Name != "Prj" {
		t.Fatalf("l=%d p=%d g=%d %+v %+v", len(l), len(p), len(g), l, p)
	}
	r2 := skills.NewRegistry()
	r2.Global["x"] = skills.SkillEntry{Name: "Glob", SourceRepo: "g"}
	l, p, g = skills.PartitionInstalledSkills(r2, projHex, projRoot)
	if len(l) != 0 || len(p) != 0 || len(g) != 1 {
		t.Fatalf("g=%d", len(g))
	}
}

func TestParseSkillFrontMatter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	content := "---\nname: MySkill\n---\nbody\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	fm, err := skills.ParseSkillFrontMatter(p)
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "MySkill" {
		t.Fatalf("%v", fm)
	}
	body, err := skills.SkillMarkdownBody(p)
	if err != nil {
		t.Fatal(err)
	}
	if body != "body" {
		t.Fatalf("body %q want no front matter", body)
	}
}

func TestRepoOwner(t *testing.T) {
	if o := skills.RepoOwner("https://github.com/acme/cool"); o != "acme" {
		t.Fatalf("got %q", o)
	}
}

func TestUniqueDisplayName(t *testing.T) {
	r := skills.NewRegistry()
	k1 := skills.StableKeyHex("https://github.com/a/x", ".")
	k2 := skills.StableKeyHex("https://github.com/b/y", ".")
	r.Global[k1] = skills.SkillEntry{Name: "Docs"}
	got := skills.UniqueDisplayName(r, "https://github.com/b/y", "Docs", skills.ScopeGlobal, "", k2)
	if want := "b-Docs"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	r.Global[k2] = skills.SkillEntry{Name: "b-Docs"}
	k3 := skills.StableKeyHex("https://github.com/c/z", ".")
	got2 := skills.UniqueDisplayName(r, "https://github.com/c/z", "Docs", skills.ScopeGlobal, "", k3)
	if got2 != "c-Docs" {
		t.Fatalf("got %q want c-Docs", got2)
	}
	r.Global[k3] = skills.SkillEntry{Name: "c-Docs"}
	k4 := skills.StableKeyHex("https://github.com/c/w", ".")
	got3 := skills.UniqueDisplayName(r, "https://github.com/c/w", "Docs", skills.ScopeGlobal, "", k4)
	if got3 != "c-Docs-2" {
		t.Fatalf("got %q want c-Docs-2", got3)
	}
}

func TestSkillHelpCommand(t *testing.T) {
	if g, w := skills.SkillHelpCommand("PRD Review"), "/PRD-Review"; g != w {
		t.Fatalf("%q want %q", g, w)
	}
	if g := skills.SkillHelpCommand(""); g != "/skill" {
		t.Fatalf("%q", g)
	}
}

func TestDescriptionFromFrontMatter(t *testing.T) {
	if d := skills.DescriptionFromFrontMatter(map[string]any{"description": "  hi  "}); d != "hi" {
		t.Fatalf("%q", d)
	}
	if d := skills.DescriptionFromFrontMatter(map[string]any{"summary": "s"}); d != "s" {
		t.Fatalf("%q", d)
	}
}

func TestAssignSkillSlash_reservedBuiltin(t *testing.T) {
	refs := []skills.SkillRefWithKey{{RegistryKey: "k1", Entry: skills.SkillEntry{Name: "plan"}}}
	b := skills.AssignSkillSlashCommands(refs)
	if len(b) != 1 || b[0].Slash != "skill-plan" {
		t.Fatalf("%+v", b)
	}
}

func TestAssignSkillSlash_duplicateNames(t *testing.T) {
	refs := []skills.SkillRefWithKey{
		{RegistryKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Entry: skills.SkillEntry{Name: "dup"}},
		{RegistryKey: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Entry: skills.SkillEntry{Name: "dup"}},
	}
	b := skills.AssignSkillSlashCommands(refs)
	if len(b) != 2 || b[0].Slash != "dup" || b[1].Slash != "skill-dup" {
		t.Fatalf("%v %v %+v", b[0].Slash, b[1].Slash, b)
	}
}

func TestParseRemoveArgs(t *testing.T) {
	_, err := skills.ParseRemoveArgs([]string{"skill"})
	if err == nil {
		t.Fatal("want err")
	}
	n, err := skills.ParseRemoveArgs([]string{"skill", "My", "Skill"})
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
	reg := skills.NewRegistry()
	k := "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdab"
	reg.Global[k] = skills.SkillEntry{Name: skillName, ClonePath: skillDir}
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
	opts := skills.RemoveOpts{ProjHex: "00", ProjRoot: projRoot, Args: []string{"skill", skillName}}
	if err := skills.RunRemove(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatal("clone dir should be gone")
	}
	b2, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatal(err)
	}
	var r2 skills.Registry
	if err := json.Unmarshal(b2, &r2); err != nil {
		t.Fatal(err)
	}
	if len(r2.Global) != 0 {
		t.Fatalf("global map: %v", r2.Global)
	}
}

func TestWithRegistryLockHappyPath(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "registry.lock")
	registryPath := filepath.Join(tmp, "registry.json")
	var called bool
	err := skills.WithRegistryLock(lockPath, registryPath, func(r *skills.Registry) error {
		if r == nil {
			t.Fatal("expected non-nil registry")
		}
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("callback was not invoked")
	}
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Fatal("registry file does not exist")
	}
}
