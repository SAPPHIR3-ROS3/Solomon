package prompt

import (
	"context"
	_ "embed"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
)

//go:embed templates/agent.tmpl
var agentRaw string

//go:embed templates/chat.tmpl
var chatRaw string

//go:embed templates/title.tmpl
var titleRaw string

//go:embed templates/summarize.tmpl
var summarizeRaw string

//go:embed templates/summarize_system.tmpl
var summarizeSystemRaw string

//go:embed templates/images.tmpl
var imagesWorkflowRaw string

//go:embed templates/atmention.tmpl
var atMentionWorkflowRaw string

//go:embed templates/btw.tmpl
var btwRaw string

//go:embed templates/btw_system.tmpl
var btwSystemRaw string

var (
	imagesWorkflowOnce sync.Once
	imagesWorkflowText string
	atMentionOnce      sync.Once
	atMentionText      string
)

func ImagesWorkflowSection() string {
	imagesWorkflowOnce.Do(func() {
		imagesWorkflowText = strings.TrimSpace(templateRaw("images"))
	})
	return imagesWorkflowText
}

func AtMentionWorkflowSection() string {
	atMentionOnce.Do(func() {
		atMentionText = strings.TrimSpace(templateRaw("atmention"))
	})
	return atMentionText
}

func templateRaw(name string) string {
	s, err := TemplateContent(name)
	if err != nil {
		if emb, ok := EmbeddedTemplate(name); ok {
			return emb
		}
		return ""
	}
	return s
}

type Data struct {
	Tools                 string
	Syntax                string
	LegacySyntax          string
	LegacyToolsEnabled    bool
	LegacyToolsForced     bool
	ExternalToolBridge    bool
	ExtraRules            string
	CustomRules           string
	GlobalInstructions    string
	RepoInstructions      string
	Language              string
	UserName              string
	DisableThinking       bool
	SystemOS              string
	SystemBits            string
	SystemCPUFamily       string
	SystemGOARCH          string
	WorkspaceAbsolutePath string
	Shell                 string
	ImagesWorkflow        string
	AtMentionWorkflow     string

	PlanningActive    bool
	ActivePlanName    string
	PlanImplementing  bool

	Anonymize bool
}

type TitleData struct {
	Language        string
	DisableThinking bool
}

type SummarizeData struct {
	Transcript        string
	Language          string
	DisableThinking   bool
	ImagesWorkflow    string
}

type BtwData struct {
	Transcript      string
	Question        string
	Language        string
	DisableThinking bool
}

func AnonymizeNativeToolInvocationSyntax() string {
	return strings.TrimSpace(`Native function calling: use the API tool/functions exposed for this session (names and JSON schemas match the tools below). Use API tool_calls only; do not emit standalone Tool: lines in assistant text as tool invocations.`)
}

func ExternalToolBridgeInvocationSyntax() string {
	return strings.TrimSpace(`Cursor proxy: invoke Solomon native tools by emitting a <tool_calls> XML block in your main assistant response text. The sidecar host parses the block, converts it to native tool_calls, and executes tools in Go on the real workspace. Use only exact tool names from ## Available tools below — never Cursor IDE built-ins (Read, StrReplace, Shell, Task, …).

Wrapper (Solomon canonical):

<tool_calls>
<tool name="TOOL_NAME">
<intent>brief purpose when the tool supports intent</intent>
<args>{"key":"value"}</args>
</tool>
</tool_calls>

Also accepted (normalized automatically): <tool_call>{"name":"TOOL_NAME","arguments":{...}}</tool_call> and <functioncall>{"name":"TOOL_NAME","arguments":{...}}</functioncall>.

Rules:
- Substitute TOOL_NAME with an exact name from ## Available tools (never emit the literal string TOOL_NAME).
- Put optional prose before the block; do not emit text after the closing </tool_calls>.
- Each <args> must be valid JSON matching the tool schema.
- Workspace read/edit/shell/find/MCP work belongs in orchestrate (use searchTools first when unsure which deferred SDK to call). subagent is a native tool_call only — not inside orchestrate scripts.

Examples (native):
<tool_calls>
<tool name="searchTools">
<args>{"query":"read file"}</args>
</tool>
</tool_calls>

<tool_calls>
<tool name="orchestrate">
<intent>List README then print first lines</intent>
<args>{"source":"package main\n\nimport \"fmt\"\nimport \"sdk\"\n\nfunc main() {\n\tb, err := sdk.ReadFile(\"README.md\")\n\tif err != nil { fmt.Println(err); return }\n\tfmt.Println(string(b))\n}","intent":"read README"}</args>
</tool>
</tool_calls>

<tool_calls>
<tool name="subagent">
<intent>Explore auth package</intent>
<args>{"sysPromptPath":"agent.tmpl","task":"Map login flow under internal/auth"}</args>
</tool>
</tool_calls>

<tool_calls>
<tool name="switchMode">
<args>{"mode":"chat"}</args>
</tool>
</tool_calls>

<tool_calls>
<tool name="searchSkill">
<args>{"query":"pull request review"}</args>
</tool>
</tool_calls>

<tool_calls>
<tool name="loadSkill">
<args>{"name":"babysit"}</args>
</tool>
</tool_calls>`)
}

func NativeToolInvocationSyntax(legacyFallbackEnabled bool) string {
	first := "Native function calling: use the API tool/functions exposed for this session (names and JSON schemas match the tools below). Use API tool_calls only; do not emit standalone Tool: lines in assistant text unless legacy text fallback is explicitly enabled for this chat."
	if !legacyFallbackEnabled {
		first = "Native function calling: use the API tool/functions exposed for this session (names and JSON schemas match the tools below). Use API tool_calls only; do not emit standalone Tool: lines in assistant text as tool invocations."
	}
	return strings.TrimSpace(first + `

The harness may echo executed calls as lines like:

Tool: TOOL_NAME({JSON_OBJECT})

for readability after execution; those echoes are not a substitute for native tool_calls when the API supports them.
`)
}

func LegacyOnlyToolInvocationSyntax(planningActive bool) string {
	return strings.TrimSpace(`
Legacy text tools force is ON. Native API tool_calls are disabled; you must invoke every tool via XML (no function calling):
` + legacyToolInvocationSyntaxBody(planningActive))
}

func LegacyToolInvocationSyntaxAppend(planningActive bool) string {
	return strings.TrimSpace(`
Optional legacy text tools are enabled: you may invoke tools with native API tool_calls (preferred) or wrap invocations in XML when helpful:
` + legacyToolInvocationSyntaxBody(planningActive))
}

func ToolInvocationSyntaxSection(legacyEnabled, legacyForced, planningActive bool) string {
	if legacyForced {
		return LegacyOnlyToolInvocationSyntax(planningActive)
	}
	if legacyEnabled {
		return strings.TrimSpace(NativeToolInvocationSyntax(true) + "\n\n" + LegacyToolInvocationSyntaxAppend(planningActive))
	}
	return ""
}

func legacyToolInvocationSyntaxCommon() string {
	return `

Preferred wrapper (Solomon canonical):

<tool_calls>
<tool name="TOOL_NAME">
<intent>brief purpose when the tool supports intent</intent>
<args>{"key":"value"}</args>
</tool>
</tool_calls>

Also accepted (normalized automatically before execution):
- Qwen-style JSON in tags: <tool_call>{"name":"TOOL_NAME","arguments":{...}}</tool_call>
- Glaive-style: <functioncall>{"name":"TOOL_NAME","arguments":{...}}</functioncall>
- Mixed Solomon + misspelled closers: use <tool name="..."> with <args>{...}</args> and close with </tool> (not </tool_call>); do not nest <tool_call> around <tool name="...">.

Rules:
- In the skeleton above, TOOL_NAME is syntax only, not a callable tool; substitute an exact name from ## Available tools below (never emit the literal string TOOL_NAME).
- Solomon parses tool XML only from your main assistant response text, not from reasoning or thinking. You may plan tool use there, but only XML in the response body is executed.
- Use exactly one tool-invocation region per assistant reply that calls tools (prefer one <tool_calls> wrapper).
- Put optional prose before the block; do not emit text after the closing tag (</tool_calls> or last </tool> / </tool_call>).
- Each <args> must be a valid JSON object matching the tool schema (for JSON-in-<tool_call>, put arguments in the "arguments" or "args" field).
- Multiple tools: include multiple <tool> entries (or multiple <tool_call> JSON objects) in order of execution.
`
}

func legacyToolInvocationSyntaxExamplesPlan() string {
	return `
Examples (PLAN):
<tool_calls>
<tool name="createPlan">
<args>{"name": "feature.md", "planText": "# Goal\n\n## Steps\n1. ..."}</args>
</tool>
</tool_calls>

Examples (PLAN):
<tool_calls>
<tool name="editPlan">
<intent>Reorder first step</intent>
<args>{"name": "feature.md", "old": "## Steps\n1. A", "new": "## Steps\n1. B"}</args>
</tool>
</tool_calls>
`
}

func legacyToolInvocationSyntaxExamplesDeferred() string {
	return `
Examples (BUILD):
<tool_calls>
<tool name="readFile">
<args>{"path": "cmd/app/main.go"}</args>
</tool>
</tool_calls>

Examples (BUILD):
<tool_calls>
<tool name="shell">
<intent>Run full test suite</intent>
<args>{"command": "go test ./..."}</args>
</tool>
</tool_calls>

Examples (BUILD):
<tool_calls>
<tool name="editFile">
<intent>Fix variable name</intent>
<args>{"path": "cmd/app/main.go", "oldString": "foo", "newString": "bar"}</args>
</tool>
</tool_calls>

Examples (BUILD):
<tool_calls>
<tool name="editFile">
<intent>Remove obsolete helper</intent>
<args>{"path": "internal/legacy/helper.go", "delete": true}</args>
</tool>
</tool_calls>

Examples (BUILD):
<tool_calls>
<tool name="find">
<args>{"pattern": "**/*.go", "files": true}</args>
</tool>
</tool_calls>

Examples (BUILD):
<tool_calls>
<tool name="find">
<args>{"pattern": "RegisterTool", "files": false, "pathGlob": "*.go"}</args>
</tool>
</tool_calls>

Examples (BUILD, multiple tools):
<tool_calls>
<tool name="shell">
<intent>Run unit tests</intent>
<args>{"command": "go test ./internal/..."}</args>
</tool>
<tool name="readFile">
<intent>Inspect config</intent>
<args>{"path": "config.toml"}</args>
</tool>
</tool_calls>
`
}

func legacyToolInvocationSyntaxBody(planningActive bool) string {
	if planningActive {
		return legacyToolInvocationSyntaxCommon() + legacyToolInvocationSyntaxExamplesPlan()
	}
	return legacyToolInvocationSyntaxCommon() + legacyToolInvocationSyntaxExamplesDeferred()
}

func RenderAgent(d Data) (string, error) {
	if d.Syntax == "" && !d.ExternalToolBridge {
		d.Syntax = NativeToolInvocationSyntax(d.LegacySyntax != "")
	}
	return render("agent", d)
}

func RenderChat(d Data) (string, error) {
	if d.Syntax == "" && !d.ExternalToolBridge {
		d.Syntax = NativeToolInvocationSyntax(d.LegacySyntax != "")
	}
	return render("chat", d)
}

func RenderTitle(d TitleData) (string, error) {
	return executeTemplate("title", templateRaw("title"), d)
}

func RenderSummarize(d SummarizeData) (string, error) {
	return executeTemplate("summarize", templateRaw("summarize"), d)
}

func RenderSummarizeSystem(d SummarizeData) (string, error) {
	d.ImagesWorkflow = ImagesWorkflowSection()
	return executeTemplate("summarize_system", templateRaw("summarize_system"), d)
}

func RenderBtw(d BtwData) (string, error) {
	return executeTemplate("btw", templateRaw("btw"), d)
}

func RenderBtwSystem(d Data) (string, error) {
	applyRuntimeSystem(&d)
	d.Shell = EffectiveShell()
	d.ImagesWorkflow = ImagesWorkflowSection()
	d.AtMentionWorkflow = AtMentionWorkflowSection()
	if d.Anonymize {
		d.ImagesWorkflow = anonymizeImagesWorkflowSection()
		d.AtMentionWorkflow = anonymizeAtMentionWorkflowSection()
	}
	s, err := executeTemplate("btw_system", templateRawExpanded("btw_system", d.WorkspaceAbsolutePath), d)
	if err != nil {
		return "", err
	}
	if d.Anonymize {
		s = sanitizeAnonymizePrompt(s)
	}
	return s, nil
}

func render(name string, d Data) (string, error) {
	applyRuntimeSystem(&d)
	d.Shell = EffectiveShell()
	d.ImagesWorkflow = ImagesWorkflowSection()
	d.AtMentionWorkflow = AtMentionWorkflowSection()
	if d.Anonymize {
		d.ImagesWorkflow = anonymizeImagesWorkflowSection()
		d.AtMentionWorkflow = anonymizeAtMentionWorkflowSection()
	}
	s, err := executeTemplate(name, templateRawExpanded(name, d.WorkspaceAbsolutePath), d)
	if err != nil {
		return "", err
	}
	if d.Anonymize {
		s = sanitizeAnonymizePrompt(s)
	}
	return s, nil
}

func templateRawExpanded(name, projRoot string) string {
	raw := templateRaw(name)
	if projRoot == "" || !strings.Contains(raw, "@") {
		return raw
	}
	out, err := atmention.ExpandDocument(context.Background(), raw, "", projRoot, nil)
	if err != nil {
		return raw
	}
	return out
}

func anonymizeImagesWorkflowSection() string {
	return strings.TrimSpace(`## Session images

The user can paste a clipboard image in the REPL with Ctrl+V (readline key 22). PNG, JPEG, or GIF bytes are stored under the chat images directory and recorded with path plus index N in session ImageFiles (N is a non-negative integer, usually 0 for the first paste in the session).

Visible paste label (REPL input and echoed user lines): a short bracketed token whose inner text is the word img, a hyphen, and the index N. The REPL buffer stores only this visible label.

Wire token (user message JSON on disk; basis for API attach): the same visible prefix, then U+200B (zero-width space), then exactly 32 Unicode private-use runes (U+E000–U+E0FF) each encoding one byte of the raw SHA-256 digest of the image file, then a closing bracket. Legacy transcripts may use 64 lowercase hex digits instead of PUA for the digest payload.

When the user sends a line, bare visible labels with a valid ImageFiles entry are expanded to the wire token. Vision is attached only if the wire digest matches the file on disk and the bytes are a recognized image; otherwise the marker is removed from the API payload (no image part and no literal marker sent upstream).

You receive image bytes in a user turn only when that attach succeeded. The same visible label appearing in assistant text, tool output, plans, quoted code, or summaries is UI or documentation—not a missing upload. Do not ask the user to re-send an image unless their current turn clearly needs a screenshot and you received no vision content.

PLAN mode cannot paste or attach images; apply the rules above when reading markers in transcript history.`)
}

func anonymizeAtMentionWorkflowSection() string {
	return strings.TrimSpace(`## @ file and folder mentions (REPL)

The user can cite workspace files or folders with @path tags in the REPL (tab completion and a path picker). Tags use the shortest project-relative path that uniquely identifies the entry (for example @c.txt or @a/b/c.txt). Tags are plain path text only — no SHA or image-style wire tokens.

When the user sends a message, each @ tag is expanded into file text or an absolute folder path in the API payload while the visible transcript keeps the short @ tags. Expansion reads the current file contents from disk at send time. If a path is missing or binary/too large, expansion notes the failure in the API payload without removing the visible tag from the transcript.

Treat @ tags in assistant text, tool output, or history as references to workspace paths, not missing uploads.`)
}

func sanitizeAnonymizePrompt(s string) string {
	r := strings.NewReplacer(
		"The Solomon process is running on:", "This session is running on:",
		"You operate in AGENT mode.", "You are a coding assistant in an interactive terminal environment.",
		"You operate in CHAT mode.", "You are a coding assistant in chat mode.",
		"Use docsRetrieval for Solomon documentation anytime.", "Use docsRetrieval for documentation when needed.",
		"docsRetrieval is always available for Solomon documentation.", "docsRetrieval is available for documentation.",
		"Prefer docsRetrieval for Solomon-specific questions", "Prefer docsRetrieval for documentation questions",
		"## Session images (Solomon paste)", "## Session images",
		"Solomon writes", "The session writes",
		"Solomon expands", "Each @ tag is expanded",
		"Solomon canonical", "canonical",
		"The harness may echo", "Executed tool calls may be echoed",
		"Deferred tools (readFile, shell, editFile, find, plan tools, …) are not exposed as separate native tool_calls — use searchTools to discover them and orchestrate to run them.", "Additional tools beyond the native API set are discovered with searchTools and run via orchestrate scripts.",
		"chains multiple deferred tool steps", "chains multiple tool steps",
		"when a task needs several deferred tools", "when a task needs several tools",
		"searching the deferred catalog", "searching the tool catalog",
		`importing the sandbox SDK (sdk, SAPPHIR3ROS3/Solomon/sdk, or SAPPHIR3ROS3/Solomon/v2026/sdk)`, `importing the sandbox SDK via import "sdk" only`,
		`importing the sandbox SDK via import "sdk" only.`, `importing the sandbox SDK via import "sdk" only`,
	)
	s = r.Replace(s)
	for _, pair := range [][2]string{
		{"SAPPHIR3ROS3/Solomon/sdk", "sdk"},
		{"SAPPHIR3ROS3/Solomon/v2026/sdk", "sdk"},
		{"SAPPHIR3-ROS3/Solomon/v2026/sdk", "sdk"},
	} {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "solomon") || strings.Contains(lower, "harness") {
		s = strings.ReplaceAll(s, "Solomon ", "")
		s = strings.ReplaceAll(s, "solomon ", "")
		s = strings.ReplaceAll(s, " harness", " session")
		s = strings.ReplaceAll(s, "Harness", "Session")
	}
	return s
}

func applyRuntimeSystem(d *Data) {
	d.SystemOS = runtime.GOOS
	d.SystemGOARCH = runtime.GOARCH
	d.SystemBits = systemBits(runtime.GOARCH)
	d.SystemCPUFamily = systemCPUFamily(runtime.GOARCH)
}

func systemBits(arch string) string {
	switch arch {
	case "386", "arm", "mips", "mipsle":
		return "32"
	case "amd64", "arm64", "loong64", "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x", "wasm":
		return "64"
	default:
		if strconv.IntSize == 32 {
			return "32"
		}
		return "64"
	}
}

func systemCPUFamily(arch string) string {
	switch arch {
	case "386", "amd64":
		return "x86"
	case "arm", "arm64":
		return "ARM"
	default:
		return "other"
	}
}

func SummarizeSystemFallback(language string) string {
	var langBlock string
	if s := strings.TrimSpace(language); s != "" {
		langBlock = "Write the summary in " + s + ". Custom rules and instruction files may be written in a language other than " + s + "; follow their intent regardless, but still write the summary in " + s + ".\n\n"
	}
	fallback := `You summarize technical conversations concisely.
Preserve important facts: decisions, file paths, commands, errors, and open tasks.
`
	if langBlock == "" {
		fallback += "Match the language of the transcript.\n"
	}
	fallback += `Output only the summary text, without preamble or meta-commentary.
Do not describe Solomon image paste markers or ask the user to re-send images unless the open task is explicitly blocked on a missing real attachment.

`
	return strings.TrimSpace(langBlock + fallback + ImagesWorkflowSection())
}

func SystemWithNoThink(disableThinking bool, content string) string {
	if !disableThinking {
		return content
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "/no_think") {
		return content
	}
	return "/no_think\n" + content
}

func executeTemplate(name, raw string, data any) (string, error) {
	t, err := template.New(name).Parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
