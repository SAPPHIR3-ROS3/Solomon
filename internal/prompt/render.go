package prompt

import (
	_ "embed"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
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

func render(name string, d Data) (string, error) {
	applyRuntimeSystem(&d)
	d.Shell = EffectiveShell()
	d.ImagesWorkflow = ImagesWorkflowSection()
	d.AtMentionWorkflow = AtMentionWorkflowSection()
	return executeTemplate(name, templateRaw(name), d)
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
