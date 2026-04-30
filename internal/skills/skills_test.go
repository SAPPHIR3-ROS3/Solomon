package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"solomon/internal/config"
	"solomon/internal/logging"
	"solomon/internal/paths"
)

func TestMain(m *testing.M) {
	logging.LogInit(logging.INFO_LOG_LEVEL)
	os.Exit(m.Run())
}

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
	p, err := ParseAddArgs([]string{"npx", "--yes", "skills", "add", "a/b", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Scope != ScopeLocal || !strings.Contains(p.NpmCommand, "skills") {
		t.Fatalf("%+v", p)
	}
	p, err = ParseAddArgs([]string{"https://skills.sh/foo/bar", "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromSkillsSh || p.Scope != ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	p, err = ParseAddArgs([]string{"skill", "https://example.com/r/skill.md", "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.RemoteMDURL != "https://example.com/r/skill.md" || p.Scope != ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	p, err = ParseAddArgs([]string{"skill", "https://example.com/r/skill.md", "Disp", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.DisplayName != "Disp" || p.Scope != ScopeProject {
		t.Fatalf("%+v", p)
	}
	if _, err := ParseAddArgs([]string{"skill", "https://example.com/r"}); err == nil {
		t.Fatal("want err for non-.md URL")
	}
	dir := t.TempDir()
	localMD := filepath.Join(dir, "loc.md")
	if err := os.WriteFile(localMD, []byte("---\nname: L\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err = ParseAddArgs([]string{"skill", localMD, "global"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromRemoteMD || p.RemoteMDURL != localMD || p.Scope != ScopeGlobal {
		t.Fatalf("%+v", p)
	}
	if _, err := ParseAddArgs([]string{"skill", "ftp://a/b.md", "global"}); err == nil {
		t.Fatal("want err for unsupported URL scheme")
	}
}

func TestIsSkillMarkdownSource(t *testing.T) {
	if !IsSkillMarkdownSource("https://a/b.md") || !IsSkillMarkdownSource("/tmp/x.md") || !IsSkillMarkdownSource("file:///tmp/x.md") {
		t.Fatal("expected accepted sources")
	}
	if IsSkillMarkdownSource("ftp://a/b.md") || IsSkillMarkdownSource("https://a/x") || IsSkillMarkdownSource("npx foo") {
		t.Fatal("expected rejected sources")
	}
}

func TestNormalizeSkillMarkdownSource_local(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := NormalizeSkillMarkdownSource(p)
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
	if _, err := NormalizeSkillMarkdownSource(filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("want err for missing file")
	}
}

func TestInstallShellCommandMeta(t *testing.T) {
	m := &SkillsShMeta{RepoURL: "https://github.com/gh/awesome-copilot", PreferredSkill: "prd"}
	got := m.InstallShellCommand()
	if !strings.Contains(got, "github.com/gh/awesome-copilot") || !strings.Contains(got, "--skill prd") ||
		!strings.Contains(got, "-g") || !strings.Contains(got, "-y") {
		t.Fatalf("%q", got)
	}
}

func TestEnsureSkillsAddGlobalYes(t *testing.T) {
	raw := "npx skills add https://github.com/a/b --skill prd"
	got := EnsureSkillsAddGlobalYes(raw)
	if !strings.Contains(got, "-g") || !strings.Contains(got, "-y") {
		t.Fatalf("%q", got)
	}
	complete := "npx --yes skills add -g -y https://github.com/a/b --skill prd"
	if EnsureSkillsAddGlobalYes(complete) != complete {
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
	r := NewRegistry()
	r.Projects[projHex] = map[string]SkillEntry{
		"a": {Name: "Loc", ClonePath: localClone, SourceRepo: "u1"},
		"b": {Name: "Prj", ClonePath: projClone, SourceRepo: "u2"},
	}
	l, p, g := partitionInstalledSkills(r, projHex, projRoot)
	if len(l) != 1 || len(p) != 1 || len(g) != 0 || l[0].Name != "Loc" || p[0].Name != "Prj" {
		t.Fatalf("l=%d p=%d g=%d %+v %+v", len(l), len(p), len(g), l, p)
	}
	r2 := NewRegistry()
	r2.Global["x"] = SkillEntry{Name: "Glob", SourceRepo: "g"}
	l, p, g = partitionInstalledSkills(r2, projHex, projRoot)
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
	fm, err := ParseSkillFrontMatter(p)
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "MySkill" {
		t.Fatalf("%v", fm)
	}
	body, err := SkillMarkdownBody(p)
	if err != nil {
		t.Fatal(err)
	}
	if body != "body" {
		t.Fatalf("body %q want no front matter", body)
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

func TestSkillHelpCommand(t *testing.T) {
	if g, w := SkillHelpCommand("PRD Review"), "/PRD-Review"; g != w {
		t.Fatalf("%q want %q", g, w)
	}
	if g := SkillHelpCommand(""); g != "/skill" {
		t.Fatalf("%q", g)
	}
}

func TestDescriptionFromFrontMatter(t *testing.T) {
	if d := DescriptionFromFrontMatter(map[string]any{"description": "  hi  "}); d != "hi" {
		t.Fatalf("%q", d)
	}
	if d := DescriptionFromFrontMatter(map[string]any{"summary": "s"}); d != "s" {
		t.Fatalf("%q", d)
	}
}

func TestAssignSkillSlash_reservedBuiltin(t *testing.T) {
	refs := []SkillRefWithKey{{RegistryKey: "k1", Entry: SkillEntry{Name: "plan"}}}
	b := AssignSkillSlashCommands(refs)
	if len(b) != 1 || b[0].Slash != "skill-plan" {
		t.Fatalf("%+v", b)
	}
}

func TestAssignSkillSlash_duplicateNames(t *testing.T) {
	refs := []SkillRefWithKey{
		{RegistryKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Entry: SkillEntry{Name: "dup"}},
		{RegistryKey: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Entry: SkillEntry{Name: "dup"}},
	}
	b := AssignSkillSlashCommands(refs)
	if len(b) != 2 || b[0].Slash != "dup" || b[1].Slash != "skill-dup" {
		t.Fatalf("%v %v %+v", b[0].Slash, b[1].Slash, b)
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

func TestSearchSkill_fallsBackToFullFileWhenDescriptionMiss(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	alphaPath := filepath.Join(home, "alpha.md")
	betaPath := filepath.Join(home, "beta.md")
	if err := WriteSkillMarkdown(alphaPath, map[string]any{"name": "Alpha", "description": "all about bananas"}, []byte("# Hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Global["k1"] = SkillEntry{
		Name:        "Alpha",
		SkillMdPath: alphaPath,
		FrontMatter: map[string]any{"name": "Alpha", "description": "all about bananas"},
	}
	reg.Global["k2"] = SkillEntry{
		Name:        "Beta",
		SkillMdPath: betaPath,
		FrontMatter: map[string]any{"name": "Beta", "description": "all about oranges"},
	}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := SearchBestInstalledSkill("zebra", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "Beta" {
		t.Fatalf("after description miss and full-file pass: want Beta, got %q (score=%v)", h.Name, h.Score)
	}
}

func TestSearchSkill_meetsDefaultNormalizedThreshold(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	alphaPath := filepath.Join(home, "alpha.md")
	betaPath := filepath.Join(home, "beta.md")
	if err := WriteSkillMarkdown(alphaPath, map[string]any{"name": "Alpha", "description": "all about bananas"}, []byte("# Hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Global["k1"] = SkillEntry{
		Name:        "Alpha",
		SkillMdPath: alphaPath,
		FrontMatter: map[string]any{"name": "Alpha", "description": "all about bananas"},
	}
	reg.Global["k2"] = SkillEntry{
		Name:        "Beta",
		SkillMdPath: betaPath,
		FrontMatter: map[string]any{"name": "Beta", "description": "all about oranges"},
	}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	h, err := SearchBestInstalledSkill("zebra", "", "", config.DefaultSkillSearchMinNormalizedScore)
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "Beta" || h.Score < config.DefaultSkillSearchMinNormalizedScore {
		t.Fatalf("want Beta and score >= default min, got %+v err=%v", h, err)
	}
}

func TestSearchSkill_noMatchWhenScoreZero(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, "only.md")
	if err := WriteSkillMarkdown(p, map[string]any{"name": "Only", "description": "apples"}, []byte("# x\n")); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Global["k"] = SkillEntry{
		Name:        "Only",
		SkillMdPath: p,
		FrontMatter: map[string]any{"description": "apples"},
	}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = SearchBestInstalledSkill("zebraxyzunique", "", "", 0)
	if err == nil {
		t.Fatal("want error when no term matches description or file")
	}
}

func TestSearchSkill_minNormFilters(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	betaPath := filepath.Join(home, "beta.md")
	if err := WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Global["k2"] = SkillEntry{
		Name:        "Beta",
		SkillMdPath: betaPath,
		FrontMatter: map[string]any{"name": "Beta", "description": "all about oranges"},
	}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = SearchBestInstalledSkill("zebra", "", "", 0.99)
	if err == nil {
		t.Fatal("want error when normalized score is below minNorm")
	}
}

func TestResolveSkillForLoad_slashAndName(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, "s.md")
	if err := WriteSkillMarkdown(p, map[string]any{"name": "Gamma Skill", "description": "d"}, []byte("body line")); err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	reg.Global["gk"] = SkillEntry{
		Name:        "Gamma Skill",
		SkillMdPath: p,
		FrontMatter: map[string]any{"description": "d"},
	}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	refs := orderedSkillRefs(reg, "", "")
	binds := AssignSkillSlashCommands(refs)
	if len(binds) != 1 {
		t.Fatalf("binds: %+v", binds)
	}
	slashTok := binds[0].Slash
	e1, s1, err := ResolveSkillForLoad("Gamma Skill", "", "")
	if err != nil || s1 != slashTok || e1.Name != "Gamma Skill" {
		t.Fatalf("%v %q %+v", err, s1, e1)
	}
	e2, s2, err := ResolveSkillForLoad("/"+slashTok, "", "")
	if err != nil || s2 != slashTok || e2.Name != "Gamma Skill" {
		t.Fatalf("%v %q %+v", err, s2, e2)
	}
}
