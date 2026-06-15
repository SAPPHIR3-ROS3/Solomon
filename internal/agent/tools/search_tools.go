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
}

func deferredCatalog() []deferredTool {
	return []deferredTool{
		{Name: "docsRetrieval", Description: "Search embedded Solomon documentation (snippets or full article by path)", SDKCall: "DocsRetrieval/DocsSearch/DocsArticle (JSON string); DocsRetrievalInfo/... → DocsResult", Mode: "both"},
		{Name: "createPlan", Description: "Create a structured plan file with frontmatter and Goal section", SDKCall: "CreatePlan(name, goal string) (map, error)", Mode: "agent"},
		{Name: "editPlan", Description: "Replace the first occurrence of old text in a plan file", SDKCall: "EditPlan(name, old, new, intent string) (map, error)", Mode: "agent"},
		{Name: "buildPlan", Description: "Prepare structured implementation brief from a plan (no nested run)", SDKCall: "BuildPlan(name string) (map, error)", Mode: "agent"},
		{Name: "addTodo", Description: "Append an open todo as the last line of the plan file", SDKCall: "AddTodo(name, todo string) (map, error)", Mode: "agent"},
		{Name: "todoList", Description: "List open todos for a plan", SDKCall: "TodoList(name string) (map[string]string, error)", Mode: "agent"},
		{Name: "checkTodo", Description: "Mark a todo done by SHA1", SDKCall: "CheckTodo(sha1 string) (map, error)", Mode: "agent"},
		{Name: "removeTodo", Description: "Remove a todo line by SHA1", SDKCall: "RemoveTodo(sha1 string) (map, error)", Mode: "agent"},
		{Name: "checkPlan", Description: "Inspect plan status and remaining todos or full body", SDKCall: "CheckPlan(name string, full bool) (map, error)", Mode: "agent"},
		{Name: "deletePlan", Description: "Delete a plan file", SDKCall: "DeletePlan(name string) (map, error)", Mode: "agent"},
		{Name: "shell", Description: "Run a shell command in the project workspace; returns combined stdout/stderr and non-zero exit as error", SDKCall: "Shell(command, intent string) (string, error); ShellWithTimeout; ShellResult/ShellResultWithTimeout → ShellOutput", Mode: "build"},
		{Name: "readFile", Description: "Read a text file relative to project root; optional startLine/endLine (1-based, inclusive)", SDKCall: "ReadFile; ReadFileLines/ReadFileLinesInfo; ReadFileFromLine; ReadFileUntilLine; ReadFileInfo → ReadResult", Mode: "build"},
		{Name: "editFile", Description: "Replace oldString once with newString; empty oldString creates/overwrites; delete=true removes; renameTo moves/renames", SDKCall: "ReplaceInFile/WriteFile/DeleteFile/RenameFile/EditFile (error); *Result variants → EditResult", Mode: "build"},
		{Name: "find", Description: "Search files by glob (files=true) or content regexp (files=false)", SDKCall: "Glob*/Grep*/GrepLines*/GrepCountEntries*; FindInfo/FindInInfo/FindTimeoutInfo → FindResult", Mode: "build"},
		{Name: "fetchWeb", Description: "Fetch URL content as markdown", SDKCall: "FetchWeb; FetchWebWithTimeout; FetchWebInfo/FetchWebInfoWithTimeout → FetchWebResult", Mode: "build"},
		{Name: "webSearch", Description: "Web search via configured engine", SDKCall: "WebSearch (JSON string); WebSearchInfo/WebSearchNInfo/... → WebSearchResult", Mode: "build"},
	}
}

func sdkQuickReference() map[string]any {
	return map[string]any{
		"imports": compile.SDKImportPathsForModel,
		"script_shape": "package main with func main(); compile errors if source is incomplete",
		"stdout":         "fmt.Print/Println/Printf output is captured and returned in orchestrate tool result field output",
		"examples": []string{
			`content, err := sdk.ReadFile("TODO.md")`,
			`r, err := sdk.ReadFileLinesInfo("main.go", 10, 50)`,
			`paths, err := sdk.Glob("**/*.go")`,
			`err := sdk.WriteFile("f.txt", "hello", "create file")`,
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
	hay := strings.ToLower(t.Name + " " + t.Description + " " + t.SDKCall + " " + t.Mode)
	if strings.Contains(hay, q) {
		return true
	}
	words := strings.Fields(q)
	if len(words) > 0 {
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	re, err := regexp.Compile(q)
	if err != nil {
		return false
	}
	return re.MatchString(hay)
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
