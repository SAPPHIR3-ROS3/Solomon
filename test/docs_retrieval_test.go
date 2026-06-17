package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/docs"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func docOpts() docs.Options {
	return docs.Options{
		MinNormalizedScore: config.DefaultDocSearchMinNormalizedScore,
		FullArticleScore:   config.DefaultDocSearchFullArticleScore,
	}
}

func TestDocsRetrieval_pathArticle(t *testing.T) {
	res, err := docs.Retrieve("user-guide/configuration.md", docOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != "article" {
		t.Fatalf("mode=%q", res.Mode)
	}
	if res.Path != "user-guide/configuration.md" {
		t.Fatalf("path=%q", res.Path)
	}
	if !strings.Contains(res.Content, "config.toml") {
		t.Fatalf("missing config.toml in content")
	}
}

func TestDocsRetrieval_readmeQueryResolvesDocsIndex(t *testing.T) {
	res, err := docs.Retrieve("README.md", docOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != "article" || res.Path != "docs-index.md" {
		t.Fatalf("got mode=%q path=%q", res.Mode, res.Path)
	}
	if !strings.Contains(res.Content, "documentation") {
		t.Fatalf("unexpected docs index content")
	}
}

func TestDocsRetrieval_genericSnippets(t *testing.T) {
	res, err := docs.Retrieve("legacy XML tools configuration", docOpts())
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != "snippets" {
		t.Fatalf("mode=%q", res.Mode)
	}
	if len(res.Results) == 0 {
		t.Fatal("no snippets")
	}
	found := false
	for _, r := range res.Results {
		if strings.Contains(r.Path, "configuration.md") || strings.Contains(r.Snippet, "legacy") {
			found = true
		}
		if len(r.Snippet) > 403 {
			t.Fatalf("snippet too long: %d", len(r.Snippet))
		}
	}
	if !found {
		t.Fatalf("results=%+v", res.Results)
	}
}

func TestDocsRetrieval_toolExec(t *testing.T) {
	args, err := json.Marshal(map[string]any{"query": "architecture/runtime-repl.md"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tools.Exec(t.Context(), &tools.Env{Cfg: config.EmptyRoot()}, "agent", tooling.Invocation{Name: "docsRetrieval", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	res, ok := out.(*docs.RetrievalResult)
	if !ok {
		t.Fatalf("type %T", out)
	}
	if res.Mode != "article" {
		t.Fatalf("mode=%q", res.Mode)
	}
}

func TestSlashDispatch_docsVisibleAndAPIContent(t *testing.T) {
	var visible, api string
	d := testDeps(nil)
	d.SubmitVisibleUserMessage = func(v, a string) error { visible, api = v, a; return nil }
	if err := agent.SlashDispatch(d, "/docs user-guide/configuration.md"); err != nil {
		t.Fatal(err)
	}
	if visible != "/docs user-guide/configuration.md" {
		t.Fatalf("visible=%q", visible)
	}
	if !strings.Contains(api, "config.toml") {
		t.Fatalf("api=%q", api)
	}
}

func TestNativeToolParams_docsRetrievalFirst(t *testing.T) {
	params, err := tools.NativeToolParams("agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(params) == 0 || params[0].OfFunction == nil {
		t.Fatal("missing first tool")
	}
	if params[0].OfFunction.Function.Name != "docsRetrieval" {
		t.Fatalf("first tool=%q", params[0].OfFunction.Function.Name)
	}
}
