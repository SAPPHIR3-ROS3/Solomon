package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/skills"
)

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
	if err := skills.WriteSkillMarkdown(alphaPath, map[string]any{"name": "Alpha", "description": "all about bananas"}, []byte("# Hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := skills.WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["k1"] = skills.SkillEntry{
		Name:        "Alpha",
		SkillMdPath: alphaPath,
		FrontMatter: map[string]any{"name": "Alpha", "description": "all about bananas"},
	}
	reg.Global["k2"] = skills.SkillEntry{
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
	h, err := skills.SearchBestInstalledSkill("zebra", "", "", 0)
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
	if err := skills.WriteSkillMarkdown(alphaPath, map[string]any{"name": "Alpha", "description": "all about bananas"}, []byte("# Hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := skills.WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["k1"] = skills.SkillEntry{
		Name:        "Alpha",
		SkillMdPath: alphaPath,
		FrontMatter: map[string]any{"name": "Alpha", "description": "all about bananas"},
	}
	reg.Global["k2"] = skills.SkillEntry{
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
	h, err := skills.SearchBestInstalledSkill("zebra", "", "", config.DefaultSkillSearchMinNormalizedScore)
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
	if err := skills.WriteSkillMarkdown(p, map[string]any{"name": "Only", "description": "apples"}, []byte("# x\n")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["k"] = skills.SkillEntry{
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
	_, err = skills.SearchBestInstalledSkill("zebraxyzunique", "", "", 0)
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
	if err := skills.WriteSkillMarkdown(betaPath, map[string]any{"name": "Beta", "description": "all about oranges"}, []byte("# Body\n\nzebra taxonomy deep dive.\n")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["k2"] = skills.SkillEntry{
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
	_, err = skills.SearchBestInstalledSkill("zebra", "", "", 0.99)
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
	if err := skills.WriteSkillMarkdown(p, map[string]any{"name": "Gamma Skill", "description": "d"}, []byte("body line")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["gk"] = skills.SkillEntry{
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
	refs := skills.OrderedSkillRefs(reg, "", "")
	binds := skills.AssignSkillSlashCommands(refs)
	if len(binds) != 1 {
		t.Fatalf("binds: %+v", binds)
	}
	slashTok := binds[0].Slash
	e1, s1, err := skills.ResolveSkillForLoad("Gamma Skill", "", "")
	if err != nil || s1 != slashTok || e1.Name != "Gamma Skill" {
		t.Fatalf("%v %q %+v", err, s1, e1)
	}
	e2, s2, err := skills.ResolveSkillForLoad("/"+slashTok, "", "")
	if err != nil || s2 != slashTok || e2.Name != "Gamma Skill" {
		t.Fatalf("%v %q %+v", err, s2, e2)
	}
}

func TestResolveForcedSkillCommand_longestNameWithRemainder(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	p1 := filepath.Join(home, "s1.md")
	if err := skills.WriteSkillMarkdown(p1, map[string]any{"name": "PRD", "description": "d1"}, []byte("body1")); err != nil {
		t.Fatal(err)
	}
	p2 := filepath.Join(home, "s2.md")
	if err := skills.WriteSkillMarkdown(p2, map[string]any{"name": "PRD Review", "description": "d2"}, []byte("body2")); err != nil {
		t.Fatal(err)
	}
	reg := skills.NewRegistry()
	reg.Global["k1"] = skills.SkillEntry{Name: "PRD", SkillMdPath: p1}
	reg.Global["k2"] = skills.SkillEntry{Name: "PRD Review", SkillMdPath: p2}
	raw, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	e, _, remainder, err := skills.ResolveForcedSkillCommand("/skill:PRD Review analizza questo file", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if e == nil || e.Name != "PRD Review" {
		t.Fatalf("skill=%+v", e)
	}
	if remainder != "analizza questo file" {
		t.Fatalf("remainder=%q", remainder)
	}
}
