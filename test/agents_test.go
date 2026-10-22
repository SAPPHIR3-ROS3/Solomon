package test

import (
	"bytes"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsGlobalTree(t *testing.T) {
	t.Setenv("TERM", "dumb")
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/__solomon/active-agents" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"projects":[{"name":"other-project","agents":[{"id":"root","model":"model-a","reasoning":"high","fast":true,"intent":"Inspect code","children":[{"id":"child","model":"model-b","reasoning":"low","fast":false,"intent":"Run tests"}]}]}]}`))
	}))
	defer server.Close()
	directory := filepath.Join(home, "run", "server")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(map[string]string{"url": server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "state.json"), state, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := commands.Agents(commands.Deps{Out: &out, ProjRoot: "unrelated"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"root model-a (high fast=on) in other-project Inspect code", "  child model-b (low fast=off) in other-project Run tests"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %q", want, out.String())
		}
	}
}

func TestAgentsFastEmojiFallback(t *testing.T) {
	fast := true
	for _, tc := range []struct{ name, term, locale, want string }{
		{"utf8", "xterm-256color", "en_US.UTF-8", "(high ⚡)"},
		{"dumb", "dumb", "en_US.UTF-8", "(high fast=on)"},
		{"ascii", "xterm", "C", "(high fast=on)"},
		{"unknown", "", "en_US.UTF-8", "(high fast=on)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TERM", tc.term)
			t.Setenv("LC_ALL", tc.locale)
			var out bytes.Buffer
			commands.PrintAgent(commands.Deps{Out: &out}, commands.AgentNode{ID: "agent", Model: "model", Reasoning: "high", Fast: &fast}, "project", 0)
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output = %q; want %q", out.String(), tc.want)
			}
		})
	}
}

func TestAgentsFastYellow(t *testing.T) {
	t.Setenv("TERM", "xterm")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	var out bytes.Buffer
	termcolor.Init(termcolor.InitOptions{Out: &out, ForceColor: true})
	defer termcolor.Init(termcolor.InitOptions{NoColor: true})
	fast := true
	commands.PrintAgent(commands.Deps{Out: &out}, commands.AgentNode{Fast: &fast}, "project", 0)
	if !strings.Contains(out.String(), "[93m⚡︎") {
		t.Fatalf("missing yellow lightning: %q", out.String())
	}
}
