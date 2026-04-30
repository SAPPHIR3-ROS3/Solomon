package test

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"solomon/internal/agent"
	"solomon/internal/chatstore"
	"solomon/internal/prompt"
)

func TestSlashDispatch_emptyWhitespace(t *testing.T) {
	d := testDeps(nil)
	for _, line := range []string{"", "   ", "\t"} {
		if err := agent.SlashDispatch(d, line); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
	}
}

func TestSlashDispatch_planBuildClear(t *testing.T) {
	var mode string
	sess := &chatstore.Session{}
	d := testDeps(sess)
	d.SetMode = func(m string) { mode = m }
	if err := agent.SlashDispatch(d, "/plan"); err != nil || mode != "plan" {
		t.Fatalf("plan: err=%v mode=%s", err, mode)
	}
	if err := agent.SlashDispatch(d, "/build"); err != nil || mode != "build" {
		t.Fatalf("build: err=%v mode=%s", err, mode)
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

func TestSlashDispatch_new(t *testing.T) {
	sess := &chatstore.Session{
		ID:       "deadbeef",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}
	d := testDeps(sess)
	if err := agent.SlashDispatch(d, "/new"); err != nil {
		t.Fatal(err)
	}
	if sess.ID != "" || len(sess.Messages) != 0 {
		t.Fatalf("want fresh session, got id=%q msgs=%d", sess.ID, len(sess.Messages))
	}
}

func TestSlashDispatch_resume_last_noChats(t *testing.T) {
	d := testDeps(nil)
	err := agent.SlashDispatch(d, "/resume last")
	if err == nil || err.Error() != "no saved chats yet" {
		t.Fatalf("got %v", err)
	}
}

func TestSlashDispatch_summarizeEmpty(t *testing.T) {
	sess := &chatstore.Session{}
	d := testDeps(sess)
	err := agent.SlashDispatch(d, "/summarize")
	if err == nil || err.Error() != "no messages to summarize" {
		t.Fatalf("got %v", err)
	}
}

func TestSlashDispatch_help(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	d := testDeps(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/help"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "/plan") || !strings.Contains(out, "/resume") || !strings.Contains(out, "/new") || !strings.Contains(out, "/exec") || !strings.Contains(out, "/legacytools") || !strings.Contains(out, "/add") || !strings.Contains(out, "/skills") || !strings.Contains(out, "/remove skill") {
		t.Fatalf("/help unexpected: %.200s", out)
	}
}

func TestSlashDispatch_exec_quoted(t *testing.T) {
	var got string
	d := testDeps(nil)
	d.SubmitUserMessage = func(s string) error { got = s; return nil }
	if err := agent.SlashDispatch(d, `/exec "one two"`); err != nil {
		t.Fatal(err)
	}
	if got != "one two" {
		t.Fatalf("got %q", got)
	}
}

func TestSlashDispatch_exec_escapeQuote(t *testing.T) {
	var got string
	d := testDeps(nil)
	d.SubmitUserMessage = func(s string) error { got = s; return nil }
	if err := agent.SlashDispatch(d, `/exec "say \"hi\""`); err != nil {
		t.Fatal(err)
	}
	if got != `say "hi"` {
		t.Fatalf("got %q", got)
	}
}

func TestSlashDispatch_exec_noDeps(t *testing.T) {
	err := agent.SlashDispatch(testDeps(nil), `/exec "x"`)
	if err == nil || err.Error() != "/exec unavailable" {
		t.Fatalf("got %v", err)
	}
}

func TestSlashDispatch_addUsage(t *testing.T) {
	err := agent.SlashDispatch(testDeps(nil), "/add")
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("got %v", err)
	}
}

func TestSlashDispatch_removeUsage(t *testing.T) {
	err := agent.SlashDispatch(testDeps(nil), "/remove")
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("got %v", err)
	}
}

func TestSlashDispatch_unknown(t *testing.T) {
	err := agent.SlashDispatch(testDeps(nil), "/nonesuch")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestSlashDispatch_exitQuit(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit"} {
		err := agent.SlashDispatch(testDeps(nil), cmd)
		if !errors.Is(err, agent.ErrExitChat) {
			t.Fatalf("%s: got %v", cmd, err)
		}
	}
}
