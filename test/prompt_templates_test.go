package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func TestEnsureTemplatesInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.WriteTemplateFile("plan", "legacy plan\n"); err != nil {
		t.Fatal(err)
	}
	if err := prompt.WriteTemplateFile("build", "legacy build\n"); err != nil {
		t.Fatal(err)
	}
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	for _, name := range prompt.RetiredTemplateNames {
		p := filepath.Join(home, "prompts", "templates", name+".tmpl")
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("retired template %s should be removed, stat err=%v", name, err)
		}
	}
	for _, name := range prompt.TemplateNames() {
		p := filepath.Join(home, "prompts", "templates", name+".tmpl")
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		emb, ok := prompt.EmbeddedTemplate(name)
		if !ok {
			t.Fatalf("no embedded %s", name)
		}
		disk, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(disk) != emb {
			t.Fatalf("installed %s differs from embedded", name)
		}
	}
}

func TestStartupTemplatesAcceptModification(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	restore := prompt.SetInteractiveSessionCheckForTest(func() bool { return true })
	defer restore()
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	modified := "custom agent prompt\n"
	if err := prompt.WriteTemplateFile("agent", modified); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{"agent": "old-saved-sha"}
	var buf bytes.Buffer
	if err := prompt.StartupTemplates(cfg, &buf, func(string) (string, error) {
		return "y", nil
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "agent template has been modified") {
		t.Fatalf("output: %q", buf.String())
	}
	got := cfg.PromptTemplates["agent"]
	want := prompt.SHA256Hex(modified)
	if got != want {
		t.Fatalf("sha %q want %q", got, want)
	}
}

func TestStartupTemplatesDenyResetsEmbedded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	restore := prompt.SetInteractiveSessionCheckForTest(func() bool { return true })
	defer restore()
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	if err := prompt.WriteTemplateFile("chat", "tampered\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{"chat": "savedsha"}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, func(string) (string, error) {
		return "n", nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.PromptTemplates["chat"]; ok {
		t.Fatal("expected chat sha removed from config")
	}
	emb, _ := prompt.EmbeddedTemplate("chat")
	disk, err := prompt.ReadTemplateFile("chat")
	if err != nil {
		t.Fatal(err)
	}
	if disk != emb {
		t.Fatalf("reset failed: disk len %d emb len %d", len(disk), len(emb))
	}
}

func TestStartupTemplatesPurgeRetiredConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{
		"plan":  "stale",
		"build": "stale",
		"agent": prompt.SHA256Hex(mustEmbedded(t, "agent")),
	}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.PromptTemplates["plan"]; ok {
		t.Fatal("plan sha should be purged")
	}
	if _, ok := cfg.PromptTemplates["build"]; ok {
		t.Fatal("build sha should be purged")
	}
}

func TestStartupTemplatesAcceptAll(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	restore := prompt.SetInteractiveSessionCheckForTest(func() bool { return true })
	defer restore()
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	if err := prompt.WriteTemplateFile("chat", "chat custom\n"); err != nil {
		t.Fatal(err)
	}
	if err := prompt.WriteTemplateFile("agent", "agent custom\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{
		"chat":  "stale",
		"agent": "stale",
	}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, func(string) (string, error) {
		return "a", nil
	}); err != nil {
		t.Fatal(err)
	}
	if cfg.PromptTemplates["chat"] != prompt.SHA256Hex("chat custom\n") {
		t.Fatal("chat sha not saved")
	}
	if cfg.PromptTemplates["agent"] != prompt.SHA256Hex("agent custom\n") {
		t.Fatal("agent sha not saved after acceptAll")
	}
}

func TestStartupTemplatesNonInteractiveError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	restore := prompt.SetInteractiveSessionCheckForTest(func() bool { return false })
	defer restore()
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	if err := prompt.WriteTemplateFile("agent", "tampered\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{"agent": "savedsha"}
	err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, nil)
	if err == nil {
		t.Fatal("expected error for non-interactive modified templates")
	}
	msg := err.Error()
	if !strings.Contains(msg, "agent") {
		t.Fatalf("error should name template: %q", msg)
	}
	cfgPath := filepath.Join(home, "config.toml")
	if !strings.Contains(msg, cfgPath) {
		t.Fatalf("error should include config path %q: %q", cfgPath, msg)
	}
	tplDir := filepath.Join(home, "prompts", "templates")
	if !strings.Contains(msg, tplDir) {
		t.Fatalf("error should include templates dir %q: %q", tplDir, msg)
	}
	if !strings.Contains(msg, "interactive terminal") {
		t.Fatalf("error should mention interactive terminal: %q", msg)
	}
}

func TestInstallTemplatesAutoUpgradeStaleDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.WriteTemplateFile("agent", "old embedded copy\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	if err := prompt.InstallTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	emb, _ := prompt.EmbeddedTemplate("agent")
	disk, err := prompt.ReadTemplateFile("agent")
	if err != nil {
		t.Fatal(err)
	}
	if disk != emb {
		t.Fatalf("expected auto-upgrade to embedded")
	}
}

func TestInstallTemplatesSkipsStartupPromptForStaleWithoutConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.WriteTemplateFile("chat", "old copy\n"); err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	if err := prompt.InstallTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("startup should not prompt after install sync: %v", err)
	}
}

func TestWriteHelpIncludesPromptTemplates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	var buf bytes.Buffer
	commands.WriteHelp(&buf, "", "", config.EmptyRoot())
	out := buf.String()
	if !strings.Contains(out, "Prompt templates") {
		t.Fatalf("help missing prompt templates section: %.200s", out)
	}
	if !strings.Contains(out, filepath.Join(home, "prompts", "templates")) {
		t.Fatalf("help should list templates dir: %.200s", out)
	}
	if !strings.Contains(out, "[prompt_templates]") {
		t.Fatalf("help should mention config section: %.200s", out)
	}
}

func TestStartupTemplatesSkipsReadWhenMtimeUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	content, err := prompt.ReadTemplateFile("agent")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.Stat(filepath.Join(home, "prompts", "templates", "agent.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{"agent": "wrong-sha-on-purpose"}
	cfg.PromptTemplateModTime = map[string]int64{"agent": mod.ModTime().Unix()}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if cfg.PromptTemplates["agent"] != "wrong-sha-on-purpose" {
		t.Fatalf("expected mtime fast-path to skip hash verify, got sha %q", cfg.PromptTemplates["agent"])
	}
	disk, err := prompt.ReadTemplateFile("agent")
	if err != nil {
		t.Fatal(err)
	}
	if disk != content {
		t.Fatal("disk content changed unexpectedly")
	}
}

func TestStartupTemplatesBackfillsMtimeWhenSHAStillMatches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := prompt.EnsureTemplatesInstalled(); err != nil {
		t.Fatal(err)
	}
	content, err := prompt.ReadTemplateFile("chat")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.EmptyRoot()
	cfg.PromptTemplates = map[string]string{"chat": prompt.SHA256Hex(content)}
	if err := prompt.StartupTemplates(cfg, &bytes.Buffer{}, nil); err != nil {
		t.Fatal(err)
	}
	if cfg.PromptTemplateModTime["chat"] == 0 {
		t.Fatal("expected mtime backfill")
	}
}

func TestEmbeddedSHAStable(t *testing.T) {
	emb, ok := prompt.EmbeddedTemplate("title")
	if !ok {
		t.Fatal("no title template")
	}
	a := prompt.SHA256Hex(emb)
	b := prompt.SHA256Hex(emb)
	if a != b {
		t.Fatalf("sha unstable: %q vs %q", a, b)
	}
}

func mustEmbedded(t *testing.T, name string) string {
	t.Helper()
	emb, ok := prompt.EmbeddedTemplate(name)
	if !ok {
		t.Fatalf("no embedded %s", name)
	}
	return emb
}
