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
		{Name: "shell", Description: "Run a shell command in the project workspace; returns combined stdout/stderr and non-zero exit as error. In orchestrate scripts assign the return value to a variable and fmt.Println it — output is not auto-included in the tool result.", SDKCall: "out, err := Shell(command, intent string) (string, error); fmt.Println(out); ShellWithTimeout; ShellResult/ShellResultWithTimeout → ShellOutput{Output, Exit, Intent}", Mode: "deferred", SearchTerms: "shell command bash zsh exec terminal run"},
		{Name: "readFile", Description: "Read a text file relative to project root; optional startLine/endLine (1-based, inclusive)", SDKCall: "ReadFile(path); ReadFileLines/ReadFileLinesInfo; ReadFileFromLine; ReadFileUntilLine; ReadFileInfo → ReadResult", Mode: "deferred", SearchTerms: "read file files content text open load"},
		{Name: "editFile", Description: "Replace oldString once with newString; empty oldString creates/overwrites; delete=true removes; renameTo moves/renames", SDKCall: "ReplaceInFile(path, old, new, intent string); WriteFile(path, content, intent string); DeleteFile(path, intent string); RenameFile(path, renameTo, intent string); *Result variants → EditResult", Mode: "deferred", SearchTerms: "write edit replace patch file save overwrite"},
		{Name: "find", Description: "Search files by glob (files=true) or content regexp (files=false)", SDKCall: "Glob/GlobInfo; Grep/GrepLines/GrepCountEntries; FindInfo/FindInInfo/FindTimeoutInfo → FindResult", Mode: "deferred", SearchTerms: "glob find grep search pattern files list"},
		{Name: "listDir", Description: "List files and immediate subdirectories in one directory (non-recursive)", SDKCall: "ListDir(path string); ListDirInfo(path string) → ListDirResult", Mode: "deferred", SearchTerms: "list directory folder ls dir entries files folders"},
		{Name: "tree", Description: "Render ASCII directory tree under a path", SDKCall: "Tree(path string); TreeDepth(path string, maxDepth int); TreeInfo(path string) → TreeResult", Mode: "deferred", SearchTerms: "tree directory structure hierarchy folders files ascii"},
		{Name: "fetchWeb", Description: "Fetch URL content as markdown", SDKCall: "FetchWeb; FetchWebWithTimeout; FetchWebInfo/FetchWebInfoWithTimeout → FetchWebResult", Mode: "deferred", SearchTerms: "fetch web url http download"},
		{Name: "webSearch", Description: "Web search via configured engine", SDKCall: "WebSearch (JSON string); WebSearchInfo/WebSearchNInfo/... → WebSearchResult", Mode: "deferred", SearchTerms: "web search internet query"},
		{Name: "deepResearch", Description: "Start a background deep research job with structured HTML report", SDKCall: "DeepResearch(query, category string) (map, error)", Mode: "deferred", SearchTerms: "research deep web report investigation"},
		{Name: "researchStatus", Description: "Get status of a deep research job by jobId", SDKCall: "ResearchStatus(jobID string) (map, error)", Mode: "deferred", SearchTerms: "research status job progress report"},
	}
}

func sdkQuickReference() map[string]any {
	return map[string]any{
		"imports": compile.SDKImportPathsForModel,
		"script_shape": "package main with func main(); compile errors if source is incomplete",
		"stdout":         "fmt.Print/Println/Printf output is captured and returned in orchestrate tool result field output; sdk.Shell return values are not — assign to a variable and fmt.Println it",
		"pitfalls": []string{
			"Do not embed large file bodies with markdown backticks inside Go raw string literals (`...`); read with sdk.ReadFile, transform in memory, write with sdk.WriteFile or sdk.ReplaceInFile",
			"Host shell commands use sdk.Shell(command, intent), not os/exec — orchestrate runs in WASM (no python3/zsh/bash on PATH)",
			"sdk.Shell returns (string, error); assign output to a variable and fmt.Println it — bare sdk.Shell(...) calls do not appear in orchestrate output",
			"sdk.Grep returns (string, error) — fmt.Println the string; use sdk.GrepLines for structured matches, not range over the string",
			"ReplaceInFile and WriteFile require an intent string (4th and 3rd args respectively)",
		},
		"examples": []string{
			`content, err := sdk.ReadFile("TODO.md")`,
			`r, err := sdk.ReadFileLinesInfo("main.go", 10, 50)`,
			`paths, err := sdk.Glob("**/*.go")`,
			`out, err := sdk.Grep("pattern")`,
			`fmt.Println(out)`,
			`lines, err := sdk.GrepLines("pattern")`,
			`entries, err := sdk.ListDir(".")`,
			`tree, err := sdk.Tree("internal")`,
			`err := sdk.WriteFile("f.txt", "hello", "create file")`,
			`err := sdk.ReplaceInFile("f.md", "old", "new", "replace section")`,
			`out, err := sdk.Shell("wc -m TODO.md", "count characters")`,
			`fmt.Println(out)`,
			`res, err := sdk.ShellResult("go test ./...", "run tests")`,
			`fmt.Println(res.Output)`,
			`fmt.Println(len(content))`,
		},
	}
}

func signatureSearchTools(query string) {}

type searchToolsArgs struct {
	Query string `json:"query"`
}

func searchToolsOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("searchTools", "Search deferred tools for orchestrate scripts and connected MCP native tools (MCP.<server>.<tool>). Returns descriptions, SDK signatures for deferred tools, and parameter schemas for MCP tools.", map[string]any{
		"query": map[string]any{"type": "string", "description": "Search query (matches name, description, and SDK signature text)"},
	}, []string{"query"})
}

func appendSearchToolsDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSearchTools)
	if err != nil {
		return err
	}
	b.addBlock("searchTools", "Discover deferred tools, SDK signatures for orchestrate scripts, and connected MCP native tools (MCP.<server>.<tool>).", sig)
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
	out := formatCatalog(hits)
	appendMCPSearchHits(env, qLower, out)
	return out, nil
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
