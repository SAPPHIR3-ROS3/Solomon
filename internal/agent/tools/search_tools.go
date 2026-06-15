package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/compile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/openai/openai-go/v2"
)

type deferredTool struct {
	Name        string
	Description string
	SDKCall     string
	Mode        string
	SearchTerms string
}

func deferredCatalog() []deferredTool {
	return []deferredTool{
		{Name: "docsRetrieval", Description: "Search embedded Solomon documentation (snippets or full article by path)", SDKCall: "DocsRetrieval/DocsSearch/DocsArticle (JSON string); DocsRetrievalInfo/... → DocsResult", Mode: "both", SearchTerms: "docs documentation search article"},
		{Name: "createPlan", Description: "Create a structured plan file with frontmatter and Goal section", SDKCall: "CreatePlan(name, goal string) (map, error)", Mode: "agent", SearchTerms: "plan create goal"},
		{Name: "editPlan", Description: "Replace the first occurrence of old text in a plan file", SDKCall: "EditPlan(name, old, new, intent string) (map, error)", Mode: "agent", SearchTerms: "plan edit replace"},
		{Name: "buildPlan", Description: "Prepare structured implementation brief from a plan (no nested run)", SDKCall: "BuildPlan(name string) (map, error)", Mode: "agent", SearchTerms: "plan build implement"},
		{Name: "addTodo", Description: "Append an open todo as the last line of the plan file", SDKCall: "AddTodo(name, todo string) (map, error)", Mode: "agent", SearchTerms: "plan todo add"},
		{Name: "todoList", Description: "List open todos for a plan", SDKCall: "TodoList(name string) (map[string]string, error)", Mode: "agent", SearchTerms: "plan todo list"},
		{Name: "checkTodo", Description: "Mark a todo done by SHA1", SDKCall: "CheckTodo(sha1 string) (map, error)", Mode: "agent", SearchTerms: "plan todo check done"},
		{Name: "removeTodo", Description: "Remove a todo line by SHA1", SDKCall: "RemoveTodo(sha1 string) (map, error)", Mode: "agent", SearchTerms: "plan todo remove delete"},
		{Name: "checkPlan", Description: "Inspect plan status and remaining todos or full body", SDKCall: "CheckPlan(name string, full bool) (map, error)", Mode: "agent", SearchTerms: "plan status inspect"},
		{Name: "deletePlan", Description: "Delete a plan file", SDKCall: "DeletePlan(name string) (map, error)", Mode: "agent", SearchTerms: "plan delete remove"},
		{Name: "shell", Description: "Run a shell command in the project workspace; returns combined stdout/stderr and non-zero exit as error", SDKCall: "Shell(command, intent string) (string, error); ShellWithTimeout; ShellResult/ShellResultWithTimeout → ShellOutput", Mode: "build", SearchTerms: "shell command bash zsh exec terminal run"},
		{Name: "readFile", Description: "Read a text file relative to project root; optional startLine/endLine (1-based, inclusive)", SDKCall: "ReadFile(path); ReadFileLines/ReadFileLinesInfo; ReadFileFromLine; ReadFileUntilLine; ReadFileInfo → ReadResult", Mode: "build", SearchTerms: "read file files content text open load"},
		{Name: "editFile", Description: "Replace oldString once with newString; empty oldString creates/overwrites; delete=true removes; renameTo moves/renames", SDKCall: "ReplaceInFile(path, old, new, intent string); WriteFile(path, content, intent string); DeleteFile(path, intent string); RenameFile(path, renameTo, intent string); *Result variants → EditResult", Mode: "build", SearchTerms: "write edit replace patch file save overwrite"},
		{Name: "find", Description: "Search files by glob (files=true) or content regexp (files=false)", SDKCall: "Glob/GlobInfo; Grep/GrepLines/GrepCountEntries; FindInfo/FindInInfo/FindTimeoutInfo → FindResult", Mode: "build", SearchTerms: "glob find grep search pattern files list"},
		{Name: "fetchWeb", Description: "Fetch URL content as markdown", SDKCall: "FetchWeb; FetchWebWithTimeout; FetchWebInfo/FetchWebInfoWithTimeout → FetchWebResult", Mode: "build", SearchTerms: "fetch web url http download"},
		{Name: "webSearch", Description: "Web search via configured engine", SDKCall: "WebSearch (JSON string); WebSearchInfo/WebSearchNInfo/... → WebSearchResult", Mode: "build", SearchTerms: "web search internet query"},
	}
}

func sdkQuickReference() map[string]any {
	return map[string]any{
		"imports": compile.SDKImportPathsForModel,
		"script_shape": "package main with func main(); compile errors if source is incomplete",
		"stdout":         "fmt.Print/Println/Printf output is captured and returned in orchestrate tool result field output",
		"pitfalls": []string{
			"Do not embed large file bodies with markdown backticks inside Go raw string literals (`...`); read with sdk.ReadFile, transform in memory, write with sdk.WriteFile or sdk.ReplaceInFile",
			"Host shell commands use sdk.Shell(command, intent), not os/exec — orchestrate runs in WASM (no python3/zsh/bash on PATH)",
			"ReplaceInFile and WriteFile require an intent string (4th and 3rd args respectively)",
		},
		"examples": []string{
			`content, err := sdk.ReadFile("TODO.md")`,
			`r, err := sdk.ReadFileLinesInfo("main.go", 10, 50)`,
			`paths, err := sdk.Glob("**/*.go")`,
			`err := sdk.WriteFile("f.txt", "hello", "create file")`,
			`err := sdk.ReplaceInFile("f.md", "old", "new", "replace section")`,
			`out, err := sdk.Shell("wc -m TODO.md", "count characters")`,
			`res, err := sdk.ShellResult("go test ./...", "run tests")`,
			`fmt.Println(len(content))`,
		},
	}
}

func signatureSearchTools(query string) {}

type searchToolsArgs struct {
	Query string `json:"query"`
}

func searchToolsOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("searchTools", "Search deferred tools callable from orchestrate Go scripts via the sandbox SDK. Returns tool names, descriptions, SDK call signatures when available, and a compact SDK quick reference.", map[string]any{
		"query": map[string]any{"type": "string", "description": "Search query (matches name, description, and SDK signature text)"},
	}, []string{"query"})
}

func appendSearchToolsDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSearchTools)
	if err != nil {
		return err
	}
	b.addBlock("searchTools", "Discover deferred tools and SDK signatures for orchestrate scripts. Always includes sdk quick reference (import aliases, stdout capture, Shell/EditFile parameter shapes).", sig)
	return nil
}

func execSearchTools(env *Env, raw json.RawMessage) (any, error) {
	var a searchToolsArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	q := strings.TrimSpace(a.Query)
	if q == "" {
		return nil, fmt.Errorf("searchTools: query is required")
	}
	qLower := strings.ToLower(q)
	cat := deferredCatalog()
	var hits []deferredTool
	for _, t := range cat {
		if matchDeferred(qLower, t) {
			hits = append(hits, t)
		}
	}
	return formatCatalog(hits), nil
}

func matchDeferred(q string, t deferredTool) bool {
	hay := deferredHaystack(t)
	if strings.Contains(hay, q) {
		return true
	}
	words := significantQueryWords(q)
	if len(words) > 0 {
		ok := true
		for _, w := range words {
			if !wordMatchesHay(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
		if matchedSignificantWords(hay, words) >= 2 {
			return true
		}
		if len(words) == 1 && wordMatchesHay(hay, words[0]) {
			return true
		}
	}
	re, err := regexp.Compile(q)
	if err != nil {
		return false
	}
	return re.MatchString(hay)
}

var searchStopWords = map[string]struct{}{
	"a": {}, "an": {}, "call": {}, "deferred": {}, "find": {}, "for": {}, "from": {},
	"get": {}, "list": {}, "orchestrate": {}, "script": {}, "scripts": {}, "sdk": {},
	"the": {}, "tool": {}, "tools": {}, "use": {}, "using": {}, "via": {},
}

func deferredHaystack(t deferredTool) string {
	return strings.ToLower(t.Name + " " + t.Description + " " + t.SDKCall + " " + t.Mode + " " + t.SearchTerms)
}

func significantQueryWords(q string) []string {
	words := strings.Fields(strings.ToLower(q))
	out := make([]string, 0, len(words))
	for _, w := range words {
		if _, skip := searchStopWords[w]; skip {
			continue
		}
		out = append(out, w)
	}
	return out
}

func wordMatchesHay(hay, w string) bool {
	if strings.Contains(hay, w) {
		return true
	}
	if strings.HasSuffix(w, "s") && len(w) > 2 {
		if strings.Contains(hay, w[:len(w)-1]) {
			return true
		}
	}
	return false
}

func matchedSignificantWords(hay string, words []string) int {
	n := 0
	for _, w := range words {
		if wordMatchesHay(hay, w) {
			n++
		}
	}
	return n
}

func formatCatalog(items []deferredTool) map[string]any {
	list := make([]map[string]string, 0, len(items))
	for _, t := range items {
		entry := map[string]string{
			"name": t.Name, "description": t.Description, "origin_mode": t.Mode,
		}
		if t.SDKCall != "" {
			entry["sdk_call"] = t.SDKCall
		}
		list = append(list, entry)
	}
	return map[string]any{
		"tools": list,
		"count": len(list),
		"sdk":   sdkQuickReference(),
	}
}
