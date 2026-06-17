package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func TestSlashDispatch_new(t *testing.T) {
	var ephemeral bool
	sess := &chatstore.Session{
		ID:       "deadbeef",
		Messages: []chatstore.Message{{Role: "user", Content: "hi"}},
	}
	d := testDeps(sess)
	d.SetEphemeralSession = func(v bool) { ephemeral = v }
	ephemeral = true
	if err := agent.SlashDispatch(d, "/new"); err != nil {
		t.Fatal(err)
	}
	if ephemeral {
		t.Fatal("want ephemeral=false after /new")
	}
	if sess.ID != "" || len(sess.Messages) != 0 {
		t.Fatalf("want fresh session, got id=%q msgs=%d", sess.ID, len(sess.Messages))
	}
	if sess.CheckpointLast != -1 || !sess.CheckpointCP0 {
		t.Fatalf("new chat checkpoint baseline: last=%d cp0=%v", sess.CheckpointLast, sess.CheckpointCP0)
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
	if !strings.Contains(out, "/goto") || !strings.Contains(out, "/checkpoint") || !strings.Contains(out, "/agent") || !strings.Contains(out, "/resume") || !strings.Contains(out, "/name") || !strings.Contains(out, "/new") || !strings.Contains(out, "/temp") || !strings.Contains(out, "/exec") || !strings.Contains(out, "/legacytools") || !strings.Contains(out, "/add") || !strings.Contains(out, "/skills") || !strings.Contains(out, "/skill:<name>") || !strings.Contains(out, "/remove skill") || !strings.Contains(out, "/mcp") || !strings.Contains(out, "/cleansessioncache") {
		t.Fatalf("/help unexpected: %.200s", out)
	}
}

func TestSlashDispatch_cleansessioncache_stripsAssistantImgLiterals(t *testing.T) {
	sess := &chatstore.Session{
		Messages: []chatstore.Message{{
			Role:          "assistant",
			ReasoningText: "discuss [img-0] in TODO",
			Content:       "see [img-1] in docs",
		}},
	}
	d := testDeps(sess)
	d.Out = bytes.NewBuffer(nil)
	d.PersistSession = func() error { return nil }
	if err := agent.SlashDispatch(d, "/cleansessioncache"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sess.Messages[0].ReasoningText, "[img-") || strings.Contains(sess.Messages[0].Content, "[img-") {
		t.Fatalf("img literals should be stripped: reasoning=%q content=%q", sess.Messages[0].ReasoningText, sess.Messages[0].Content)
	}
	out := d.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "stripped") {
		t.Fatalf("output should report stripped count: %q", out)
	}
}

func TestSlashDispatch_cleansessioncache_badAttachment(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "missing.png")
	sess := &chatstore.Session{
		Messages:   []chatstore.Message{{Role: "user", Content: "text [img-0]"}},
		ImageFiles: map[int]string{0: badPath},
		ID:         "deadbeef",
	}
	var persisted bool
	buf := bytes.NewBuffer(nil)
	d := testDeps(sess)
	d.Out = buf
	d.PersistSession = func() error { persisted = true; return nil }
	if err := agent.SlashDispatch(d, "/cleansessioncache"); err != nil {
		t.Fatal(err)
	}
	if !persisted {
		t.Fatal("expected persist")
	}
	if strings.Contains(sess.Messages[0].Content, "[img-0]") {
		t.Fatalf("tag should be stripped: %q", sess.Messages[0].Content)
	}
	if len(sess.ImageFiles) != 0 {
		t.Fatalf("image map cleared, got %+v", sess.ImageFiles)
	}
	out := buf.String()
	if !strings.Contains(out, "dropped") || !strings.Contains(out, "[cleansessioncache]") {
		t.Fatalf("output: %q", out)
	}
}

func TestSlashDispatch_cleansessioncache_keepsValidPath(t *testing.T) {
	tinyPNG, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAusBWFYpXQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	goodPath := filepath.Join(t.TempDir(), "ok.png")
	if err := os.WriteFile(goodPath, tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	sess := &chatstore.Session{
		Messages:   []chatstore.Message{{Role: "user", Content: "[img-0] caption"}},
		ImageFiles: map[int]string{0: goodPath},
		ID:         "cafe4242",
	}
	d := testDeps(sess)
	d.Out = bytes.NewBuffer(nil)
	d.PersistSession = func() error { return nil }
	if err := agent.SlashDispatch(d, "/cleansessioncache"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sess.Messages[0].Content, "caption") {
		t.Fatalf("want caption kept, got %q", sess.Messages[0].Content)
	}
	if !strings.Contains(sess.Messages[0].Content, "\u200b") {
		t.Fatalf("want migrated SEP token, got %q", sess.Messages[0].Content)
	}
	p, ok := sess.ImageFiles[0]
	if !ok || p != goodPath {
		t.Fatalf("map entry dropped incorrectly: %+v", sess.ImageFiles)
	}
}

func TestSlashDispatch_mcp(t *testing.T) {
	termcolor.Init(termcolor.InitOptions{Out: &bytes.Buffer{}, NoColor: true})
	dir := t.TempDir()
	p := filepath.Join(dir, "mcp.json")
	t.Setenv("SOLOMON_MCP_CONFIG", p)
	if err := os.WriteFile(p, []byte(`{"mcpServers":{"filesystem":{"command":"npx","args":["secret"],"allow":["read_file"],"deny":["write_file"]},"remote":{"url":"https://token@example.com/mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(nil)
	d := testDeps(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/mcp"); err != nil {
		t.Fatal(err)
	}
	out := termcolor.Plain(buf.String())
	if !strings.Contains(out, "filesystem\tstdio\tnpx") || !strings.Contains(out, "remote\tstreamable-http\thttps://example.com") {
		t.Fatalf("/mcp unexpected: %s", out)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "token") {
		t.Fatalf("/mcp leaked sensitive config: %s", out)
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

func TestSlashDispatch_forcedSkillVisibleAndAPIContent(t *testing.T) {
	home := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	t.Setenv("HOME", home)
	regPath := filepath.Join(home, ".solomon", "skills.json")
	if err := os.MkdirAll(filepath.Dir(regPath), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(home, "skill.md")
	if err := os.WriteFile(p, []byte("---\nname: PRD Review\ndescription: d\n---\n\nchecklist body"), 0o600); err != nil {
		t.Fatal(err)
	}
	regBody, err := json.Marshal(map[string]any{
		"global": map[string]any{
			"k": map[string]any{
				"name":          "PRD Review",
				"skill_md_path": p,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regPath, regBody, 0o600); err != nil {
		t.Fatal(err)
	}
	var visible, api string
	d := testDeps(nil)
	d.SubmitVisibleUserMessage = func(v, a string) error { visible, api = v, a; return nil }
	if err := agent.SlashDispatch(d, "/skill:PRD Review analizza questo file"); err != nil {
		t.Fatal(err)
	}
	if visible != "/skill:PRD Review analizza questo file" {
		t.Fatalf("visible=%q", visible)
	}
	if !strings.Contains(api, `Skill: "PRD Review"`) || !strings.Contains(api, "checklist body") || !strings.Contains(api, "analizza questo file") {
		t.Fatalf("api=%q", api)
	}
}

func TestSlashDispatch_forcedSkillNotFound(t *testing.T) {
	d := testDeps(nil)
	err := agent.SlashDispatch(d, "/skill:missing")
	if err == nil || !strings.Contains(err.Error(), "try /skills") {
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

func TestSlashDispatch_rulesAndInstructions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	if err := os.WriteFile(filepath.Join(home, "AGENTS.md"), []byte("hello global agents"), 0o600); err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(nil)
	d := testDeps(nil)
	d.Out = buf
	if err := agent.SlashDispatch(d, "/add rule always use gofmt"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "global rule 1 saved") {
		t.Fatalf("add rule: %q", buf.String())
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/rules"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "always use gofmt") {
		t.Fatalf("rules list: %q", buf.String())
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/instructions"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hello global agents") {
		t.Fatalf("instructions: %q", buf.String())
	}
	buf.Reset()
	if err := agent.SlashDispatch(d, "/remove rule 1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Fatalf("remove: %q", buf.String())
	}
}
