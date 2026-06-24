package test

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func TestSlashDispatch_emptyWhitespace(t *testing.T) {
	d := testDeps(nil)
	for _, line := range []string{"", "   ", "\t"} {
		if err := agent.SlashDispatch(d, line); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
	}
}

func TestSlashDispatch_terminal(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	d := testDeps(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/terminal on"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "shell-first REPL: on") {
		t.Fatalf("on: %q", buf.String())
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/terminal off"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "shell-first REPL: off") {
		t.Fatalf("off: %q", buf.String())
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/terminal"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "shell-first REPL: on") {
		t.Fatalf("toggle from off: %q", buf.String())
	}
	if err := agent.SlashDispatch(d, "/terminal bogus"); err == nil {
		t.Fatal("want usage error")
	}
}

func TestSlashDispatch_agentClear(t *testing.T) {
	var mode string
	d := testDeps(&chatstore.Session{})
	d.SetMode = func(m string) { mode = m }
	if err := agent.SlashDispatch(d, "/agent"); err != nil || mode != "agent" {
		t.Fatalf("agent: err=%v mode=%s", err, mode)
	}
	out := bytes.NewBuffer(nil)
	d.Out = out
	if err := agent.SlashDispatch(d, "/clear"); err != nil {
		t.Fatal(err)
	}
	got := out.Bytes()
	clearSeq := []byte("\033[2J\033[H")
	if runtime.GOOS == "windows" {
		sh := prompt.EffectiveShell()
		if sh != "unknown" && strings.EqualFold(filepath.Base(sh), "cmd.exe") {
			if !strings.Contains(string(got), "cmd.exe") {
				t.Fatalf("/clear on cmd: want notice, got %q", got)
			}
			return
		}
	}
	if !bytes.Equal(got, clearSeq) {
		t.Fatalf("/clear ansi got %q (%d bytes)", got, len(got))
	}
}

func TestSlashDispatch_logLevels(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/log info"); err != nil {
		t.Fatal(err)
	}
}

func TestSlashDispatch_reasoning(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/reasoning"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/reasoning low"); err != nil {
		t.Fatal(err)
	}
}

func TestSlashDispatch_fast(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/fast off"); err != nil {
		t.Fatal(err)
	}
	if d.Cfg.EffectiveFastMode() {
		t.Fatal("want fast mode off")
	}
	if err := agent.SlashDispatch(d, "/fast"); err != nil {
		t.Fatal(err)
	}
	if !d.Cfg.EffectiveFastMode() {
		t.Fatal("want fast mode toggled on")
	}
}

func TestSlashDispatch_cursortools(t *testing.T) {
	stopCursorSidecar(t)
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/cursortools on"); err == nil {
		t.Fatal("expected unknown command without Cursor API configured")
	}
	for _, n := range commands.SlashBuiltinNames(d.Cfg) {
		if n == "cursortools" {
			t.Fatal("cursortools should be hidden without Cursor API")
		}
	}

	d.Cfg.Providers[config.ProviderNameCursorAPI] = &config.Provider{
		Name:     config.ProviderNameCursorAPI,
		AuthKind: config.AuthKindCursorAPI,
		BaseURL:  "http://127.0.0.1:8766/v1/",
		APIKey:   "cursor-key",
	}
	found := false
	for _, n := range commands.SlashBuiltinNames(d.Cfg) {
		if n == "cursortools" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cursortools should appear after Cursor API setup")
	}
	if err := agent.SlashDispatch(d, "/cursortools on"); err == nil {
		t.Fatal("expected error enabling deprecated cursor native tools")
	}
	if d.Cfg.Tools.CursorInternalTools {
		t.Fatal("cursor internal tools must stay off")
	}
	if err := agent.SlashDispatch(d, "/cursortools off"); err != nil {
		t.Fatal(err)
	}
	if d.Cfg.Tools.CursorInternalTools {
		t.Fatal("want cursor internal tools off")
	}
}

func TestSlashDispatch_threshold(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	d := testDeps(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/threshold"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "131072") {
		t.Fatalf("/threshold default: %s", buf.String())
	}
	if err := agent.SlashDispatch(d, "/threshold 32767"); err == nil {
		t.Fatal("want error for threshold < 32768")
	}
	if err := agent.SlashDispatch(d, "/threshold 65536"); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/threshold"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "65536") {
		t.Fatalf("/threshold after set: %s", buf.String())
	}
}

func TestSlashDispatch_timeout_stats_thinking_max(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/timeout 10"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/stats"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/thinking"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/thinking off"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/max_response 2048"); err != nil {
		t.Fatal(err)
	}
}

func TestSlashDispatch_language_resume_list(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/language"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/language clear"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/resume"); err != nil {
		t.Fatal(err)
	}
}

func TestSlashDispatch_name(t *testing.T) {
	d := testDeps(nil)
	if err := agent.SlashDispatch(d, "/name"); err != nil {
		t.Fatal(err)
	}
	if err := agent.SlashDispatch(d, "/name Ada Lovelace"); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(d.Cfg.UserName) != "Ada Lovelace" {
		t.Fatalf("user_name=%q", d.Cfg.UserName)
	}
	if err := agent.SlashDispatch(d, "/name clear"); err != nil {
		t.Fatal(err)
	}
	if d.Cfg.UserName != "" {
		t.Fatalf("want empty after clear, got %q", d.Cfg.UserName)
	}
}

func TestSlashDispatch_temp(t *testing.T) {
	var ephemeral bool
	sess := &chatstore.Session{
		ID:       "deadbeef",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}
	d := testDeps(sess)
	d.SetEphemeralSession = func(v bool) { ephemeral = v }
	err := agent.SlashDispatch(d, "/temp")
	if err == nil || !strings.Contains(err.Error(), "already has messages") {
		t.Fatalf("non-empty: got %v", err)
	}
	sess.Messages = nil
	sess.ID = ""
	buf := bytes.NewBuffer(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/temp"); err != nil {
		t.Fatal(err)
	}
	if !ephemeral {
		t.Fatal("want ephemeral=true")
	}
	if sess.ID != "" || len(sess.Messages) != 0 {
		t.Fatalf("want fresh session, got id=%q msgs=%d", sess.ID, len(sess.Messages))
	}
	if !strings.Contains(buf.String(), "temp session") {
		t.Fatalf("banner: %q", buf.String())
	}
}

