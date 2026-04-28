package test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"solomon/internal/agent"
	"solomon/internal/chatstore"
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
	clearSeq := []byte("\033[2J\033[H")
	if err := agent.SlashDispatch(d, "/clear"); err != nil {
		t.Fatal(err)
	}
	if got := out.Bytes(); !bytes.Equal(got, clearSeq) {
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
	if !strings.Contains(out, "/plan") || !strings.Contains(out, "/resume") {
		t.Fatalf("/help unexpected: %.200s", out)
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
