package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

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
		{Name: "docsRetrieval", Description: "Search embedded Solomon documentation (snippets or full article by path)", SDKCall: "DocsRetrieval(query, intent string) (string, error); DocsSearch(query, intent string) (string, error); DocsArticle(path, intent string) (string, error); DocsRetrievalInfo(query, intent string) (DocsResult, error); DocsSearchInfo(query, intent string) (DocsResult, error); DocsArticleInfo(path, intent string) (DocsResult, error)", Mode: "both", SearchTerms: "docs documentation search article"},
		{Name: "createPlan", Description: "Create a structured plan file with frontmatter and Goal section", SDKCall: "CreatePlan(name, goal, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan create goal"},
		{Name: "editPlan", Description: "Replace the first occurrence of old text in a plan file", SDKCall: "EditPlan(name, oldString, newString, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan edit replace"},
		{Name: "buildPlan", Description: "Prepare structured implementation brief from a plan (no nested run)", SDKCall: "BuildPlan(name, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan build implement"},
		{Name: "addTodo", Description: "Append an open todo as the last line of the plan file", SDKCall: "AddTodo(name, todo, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan todo add"},
		{Name: "todoList", Description: "List open todos for a plan", SDKCall: "TodoList(name, intent string) (map[string]string, error)", Mode: "agent", SearchTerms: "plan todo list"},
		{Name: "checkTodo", Description: "Mark a todo done by SHA1", SDKCall: "CheckTodo(sha1, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan todo check done"},
		{Name: "removeTodo", Description: "Remove a todo line by SHA1", SDKCall: "RemoveTodo(sha1, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan todo remove delete"},
		{Name: "checkPlan", Description: "Inspect plan status and remaining todos or full body", SDKCall: "CheckPlan(name string, full bool, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan status inspect"},
		{Name: "deletePlan", Description: "Delete a plan file", SDKCall: "DeletePlan(name, intent string) (map[string]any, error)", Mode: "agent", SearchTerms: "plan delete remove"},
		{Name: "shell", Description: "Run a shell command in the project workspace; returns combined stdout/stderr and non-zero exit as error. In orchestrate scripts assign the return value to a variable and fmt.Println it — output is not auto-included in the tool result.", SDKCall: "Shell(command, intent string) (string, error); ShellWithTimeout(command, intent string, secs int) (string, error); ShellResult(command, intent string) (ShellOutput, error); ShellResultWithTimeout(command, intent string, secs int) (ShellOutput, error)", Mode: "deferred", SearchTerms: "shell command bash zsh exec terminal run"},
		{Name: "readFile", Description: "Read a text file relative to project root; optional startLine/endLine (1-based, inclusive)", SDKCall: "ReadFile(path, intent string) (string, error); ReadFileLines(path string, start, end int, intent string) (string, error); ReadFileFromLine(path string, start int, intent string) (string, error); ReadFileUntilLine(path string, end int, intent string) (string, error); ReadFileInfo(path, intent string) (ReadResult, error); ReadFileLinesInfo(path string, start, end int, intent string) (ReadResult, error); ReadFileFromLineInfo(path string, start int, intent string) (ReadResult, error); ReadFileUntilLineInfo(path string, end int, intent string) (ReadResult, error)", Mode: "deferred", SearchTerms: "read file files content text open load"},
		{Name: "editFile", Description: "Replace oldString once with newString; empty oldString creates/overwrites; delete=true removes; renameTo moves/renames", SDKCall: "ReplaceInFile(path, oldString, newString, intent string) error; WriteFile(path, content, intent string) error; DeleteFile(path, intent string) error; RenameFile(path, renameTo, intent string) error; EditFile(path, oldString, newString, intent string, delete bool) error; ReplaceInFileResult(path, oldString, newString, intent string) (EditResult, error); WriteFileResult(path, content, intent string) (EditResult, error); DeleteFileResult(path, intent string) (EditResult, error); RenameFileResult(path, renameTo, intent string) (EditResult, error); EditFileResult(path, oldString, newString, intent string, delete bool) (EditResult, error)", Mode: "deferred", SearchTerms: "write edit replace patch file save overwrite"},
		{Name: "find", Description: "Search files by glob (files=true) or content regexp (files=false)", SDKCall: "Find(pattern string, files bool, intent string) (string, error); FindIn(dir, pattern string, files bool, intent string) (string, error); FindTimeout(pattern string, files bool, secs int, intent string) (string, error); FindInTimeout(dir, pattern string, files bool, secs int, intent string) (string, error); FindInfo(pattern string, files bool, intent string) (FindResult, error); Glob(pattern, intent string) ([]string, error); GlobIn(dir, pattern, intent string) ([]string, error); GlobLimit(pattern string, headLimit int, intent string) ([]string, error); GlobTimeout(pattern string, secs int, intent string) ([]string, error); Grep(pattern, intent string) (string, error); GrepIn(dir, pattern, intent string) (string, error); GrepLines(pattern, intent string) ([]GrepLine, error); GrepCountEntries(pattern, intent string) ([]GrepCountEntry, error)", Mode: "deferred", SearchTerms: "glob find grep search pattern files list"},
		{Name: "listDir", Description: "List files and immediate subdirectories in one directory (non-recursive)", SDKCall: "ListDir(path, intent string) ([]ListDirEntry, error); ListDirInfo(path, intent string) (ListDirResult, error)", Mode: "deferred", SearchTerms: "list directory folder ls dir entries files folders"},
		{Name: "tree", Description: "Render ASCII directory tree under a path", SDKCall: "Tree(path, intent string) (string, error); TreeDepth(path string, maxDepth int, intent string) (string, error); TreeInfo(path, intent string) (TreeResult, error)", Mode: "deferred", SearchTerms: "tree directory structure hierarchy folders files ascii"},
		{Name: "fetchWeb", Description: "Fetch URL content as markdown", SDKCall: "FetchWeb(url, intent string) (string, error); FetchWebWithTimeout(url string, secs int, intent string) (string, error); FetchWebInfo(url, intent string) (FetchWebResult, error); FetchWebInfoWithTimeout(url string, secs int, intent string) (FetchWebResult, error)", Mode: "deferred", SearchTerms: "fetch web url http download"},
		{Name: "webSearch", Description: "Web search via configured engine", SDKCall: "WebSearch(query, intent string) (string, error); WebSearchN(query string, maxResults int, intent string) (string, error); WebSearchWithTimeout(query string, secs int, intent string) (string, error); WebSearchEngine(query, engine, intent string) (string, error); WebSearchEngineN(query, engine string, maxResults int, intent string) (string, error); WebSearchEngineTimeout(query, engine string, secs int, intent string) (string, error); WebSearchNTimeout(query string, maxResults, secs int, intent string) (string, error); WebSearchEngineNTimeout(query, engine string, maxResults, secs int, intent string) (string, error); WebSearchInfo(query, intent string) (WebSearchResult, error); WebSearchNInfo(query string, maxResults int, intent string) (WebSearchResult, error); WebSearchWithTimeoutInfo(query string, secs int, intent string) (WebSearchResult, error); WebSearchEngineInfo(query, engine, intent string) (WebSearchResult, error); WebSearchEngineNInfo(query, engine string, maxResults int, intent string) (WebSearchResult, error); WebSearchEngineTimeoutInfo(query, engine string, secs int, intent string) (WebSearchResult, error); WebSearchNTimeoutInfo(query string, maxResults, secs int, intent string) (WebSearchResult, error); WebSearchEngineNTimeoutInfo(query, engine string, maxResults, secs int, intent string) (WebSearchResult, error)", Mode: "deferred", SearchTerms: "web search internet query"},
		{Name: "deepResearch", Description: "Start a background deep research job with structured HTML report", SDKCall: "DeepResearch(query, category, intent string) (map[string]any, error)", Mode: "deferred", SearchTerms: "research deep web report investigation"},
		{Name: "researchStatus", Description: "Get status of a deep research job by jobId", SDKCall: "ResearchStatus(jobID, intent string) (map[string]any, error)", Mode: "deferred", SearchTerms: "research status job progress report"},
	}
}

func sdkQuickReference() map[string]any {
	return map[string]any{
		"imports":      compile.SDKImportPathsForModel,
		"script_shape": "package main with func main(); compile errors if source is incomplete",
		"stdout":       "fmt.Print/Println/Printf output is captured and returned in orchestrate tool result field output; sdk.Shell return values are not — assign to a variable and fmt.Println it",
		"contract": []string{
			"Import the sandbox SDK only as import \"sdk\"; do not import the canonical internal package path.",
			"Every SDK call that reaches a host tool has a final non-empty intent string.",
			"Check every returned error before using the result.",
			"Mutating helpers return only error; their *Result variants return (typedResult, error).",
			"In quoted Go strings, regular-expression backslashes must be escaped; raw strings cannot contain a backtick.",
		},
		"pitfalls": []string{
			"Do not embed large file bodies with markdown backticks inside Go raw string literals (`...`); read with sdk.ReadFile, transform in memory, write with sdk.WriteFile or sdk.ReplaceInFile",
			"Host shell commands use sdk.Shell(command, intent), not os/exec — orchestrate runs in WASM (no python3/zsh/bash on PATH)",
			"sdk.Shell returns (string, error); assign output to a variable and fmt.Println it — bare sdk.Shell(...) calls do not appear in orchestrate output",
			"sdk.Grep returns (string, error) — fmt.Println the string; use sdk.GrepLines for structured matches, not range over the string",
			"Every sdk function that performs a tool call requires a final non-empty intent string; this includes reads, searches, plans, web, research, shell, and edits",
		},
		"examples": []string{
			`content, err := sdk.ReadFile("TODO.md", "inspect the todo list")`,
			`r, err := sdk.ReadFileLinesInfo("main.go", 10, 50, "inspect the entrypoint")`,
			`paths, err := sdk.Glob("**/*.go", "find Go files")`,
			`out, err := sdk.Grep("pattern", "find matching source")`,
			`fmt.Println(out)`,
			`lines, err := sdk.GrepLines("pattern", "inspect matching lines")`,
			`entries, err := sdk.ListDir(".", "list the project root")`,
			`tree, err := sdk.Tree("internal", "inspect the internal tree")`,
			`err := sdk.WriteFile("f.txt", "hello", "create file")`,
			`err := sdk.ReplaceInFile("f.md", "old", "new", "replace section")`,
			`out, err := sdk.Shell("wc -m TODO.md", "count characters")`,
			`fmt.Println(out)`,
			`res, err := sdk.ShellResult("go test ./...", "run tests")`,
			`fmt.Println(res.Output)`,
			`result, err := sdk.mcp.search("search remote data", map[string]any{"query": "MCP"})`,
			`fmt.Println(len(content))`,
		},
		"script_template": `package main

import (
	"fmt"
	"sdk"
)

func main() {
	out, err := sdk.Shell("pwd", "inspect the workspace")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(out)
}`,
	}
}

func signatureSearchTools(query, intent string) {}

type searchToolsArgs struct {
	Query string `json:"query"`
}

func searchToolsOpenAI() openai.ChatCompletionToolUnionParam {
	return nativeToolUnion("searchTools", "Search deferred tools for orchestrate scripts and connected MCP schemas (MCP.<server>.<tool>). Returns descriptions, SDK signatures for deferred tools, and remote parameter schemas for MCP tools; invoke MCP tools from orchestrate with sdk.mcp.<tool>(intent, args), where intent is separate from the remote argument dictionary.", map[string]any{
		"query": map[string]any{"type": "string", "description": "Search query (matches name, description, and SDK signature text)"},
	}, []string{"query"})
}

func appendSearchToolsDump(b *dumpBuilder) error {
	sig, err := tooling.FuncSignature(signatureSearchTools)
	if err != nil {
		return err
	}
	b.addBlock("searchTools", "Discover deferred tools, SDK signatures for orchestrate scripts, and connected MCP schemas (MCP.<server>.<tool>). Invoke MCP tools from orchestrate with sdk.mcp.<tool>(intent, args); intent is separate from the remote argument dictionary.", sig)
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
	} else if strings.TrimSpace(q) != "" {
		// A query such as "SDK signatures" is intentionally broad: after
		// removing discovery words there is no narrower term to match.
		return true
	}
	re, err := regexp.Compile(q)
	if err != nil {
		return false
	}
	return re.MatchString(hay)
}

var searchStopWords = map[string]struct{}{
	"a": {}, "an": {}, "call": {}, "deferred": {}, "find": {}, "for": {}, "from": {},
	"api": {}, "apis": {}, "function": {}, "functions": {}, "get": {}, "helper": {}, "helpers": {},
	"list": {}, "orchestrate": {}, "result": {}, "results": {}, "return": {}, "returns": {},
	"schema": {}, "schemas": {}, "script": {}, "scripts": {}, "sdk": {}, "signature": {}, "signatures": {},
	"the": {}, "tool": {}, "tools": {}, "use": {}, "using": {}, "via": {},
}

func deferredHaystack(t deferredTool) string {
	return strings.ToLower(t.Name + " " + t.Description + " " + t.SDKCall + " " + t.Mode + " " + t.SearchTerms)
}

func significantQueryWords(q string) []string {
	words := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(".,:;()[]{}<>\"'/\\`", r)
	})
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
