package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
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
