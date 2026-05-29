package prompt

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/plan.tmpl
var planRaw string

//go:embed templates/build.tmpl
var buildRaw string

//go:embed templates/title.tmpl
var titleRaw string

//go:embed templates/summarize.tmpl
var summarizeRaw string

//go:embed templates/summarize_system.tmpl
var summarizeSystemRaw string

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
	LocalDateTime         string
}

type TitleData struct {
	Language        string
	DisableThinking bool
}

type SummarizeData struct {
	Transcript        string
	DisableThinking   bool
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

func LegacyOnlyToolInvocationSyntax(planMode bool) string {
	return strings.TrimSpace(`
Legacy text tools force is ON. Native API tool_calls are disabled; you must invoke every tool via XML (no function calling):
` + legacyToolInvocationSyntaxBody(planMode))
}

func LegacyToolInvocationSyntaxAppend(planMode bool) string {
	return strings.TrimSpace(`
Optional legacy text tools are enabled: you may invoke tools with native API tool_calls (preferred) or wrap invocations in XML when helpful:
` + legacyToolInvocationSyntaxBody(planMode))
}

func ToolInvocationSyntaxSection(legacyEnabled, legacyForced, planMode bool) string {
	if legacyForced {
		return LegacyOnlyToolInvocationSyntax(planMode)
	}
	if legacyEnabled {
		return strings.TrimSpace(NativeToolInvocationSyntax(true) + "\n\n" + LegacyToolInvocationSyntaxAppend(planMode))
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

func legacyToolInvocationSyntaxExamplesBuild() string {
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

func legacyToolInvocationSyntaxBody(planMode bool) string {
	if planMode {
		return legacyToolInvocationSyntaxCommon() + legacyToolInvocationSyntaxExamplesPlan()
	}
	return legacyToolInvocationSyntaxCommon() + legacyToolInvocationSyntaxExamplesBuild()
}

func RenderPlan(d Data) (string, error) {
	if d.Syntax == "" && !d.ExternalToolBridge {
		d.Syntax = NativeToolInvocationSyntax(d.LegacySyntax != "")
	}
	return render(planRaw, d)
}

func RenderBuild(d Data) (string, error) {
	if d.Syntax == "" && !d.ExternalToolBridge {
		d.Syntax = NativeToolInvocationSyntax(d.LegacySyntax != "")
	}
	return render(buildRaw, d)
}

func RenderTitle(d TitleData) (string, error) {
	return executeTemplate("title", titleRaw, d)
}

func RenderSummarize(d SummarizeData) (string, error) {
	return executeTemplate("summarize", summarizeRaw, d)
}

func RenderSummarizeSystem(d SummarizeData) (string, error) {
	return executeTemplate("summarize_system", summarizeSystemRaw, d)
}

func render(raw string, d Data) (string, error) {
	applyRuntimeSystem(&d)
	d.Shell = effectiveShell()
	d.LocalDateTime = time.Now().Format(time.RFC3339)
	return executeTemplate("p", raw, d)
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

func effectiveShell() string {
	if v := strings.TrimSpace(os.Getenv("SHELL")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		if sh := windowsInteractiveShellOverride(); sh != "" {
			return sh
		}
		if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
			return v
		}
		if p := windowsFallbackShellExecutable(); p != "" {
			return p
		}
		return "unknown"
	}
	if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
		return v
	}
	return "unknown"
}

func EffectiveShell() string {
	return effectiveShell()
}

func SummarizeSystemFallback() string {
	return strings.TrimSpace(`You summarize technical conversations concisely.
Preserve important facts: decisions, file paths, commands, errors, and open tasks.
Match the language of the transcript.
Output only the summary text, without preamble or meta-commentary.`)
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

func windowsFallbackShellExecutable() string {
	systemRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRoot == "" {
		systemRoot = strings.TrimSpace(os.Getenv("windir"))
	}
	if systemRoot != "" {
		for _, rel := range []string{`System32\WindowsPowerShell\v1.0\powershell.exe`, `SysWOW64\WindowsPowerShell\v1.0\powershell.exe`} {
			p := filepath.Join(systemRoot, rel)
			if isExecutableFile(p) {
				return p
			}
		}
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		p := filepath.Join(base, "PowerShell", "7", "pwsh.exe")
		if isExecutableFile(p) {
			return p
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
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
