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

type Data struct {
	Tools                 string
	Syntax                string
	ExtraRules            string
	Language              string
	UserName              string
	SystemOS              string
	SystemBits            string
	SystemCPUFamily       string
	SystemGOARCH          string
	WorkspaceAbsolutePath string
	Shell                 string
	LocalDateTime         string
}

type TitleData struct {
	Language string
}

type SummarizeData struct {
	Transcript string
}

func NativeToolInvocationSyntax() string {
	return strings.TrimSpace(`
Native function calling: use the API tool/functions exposed for this session (names and JSON schemas match the tools below). Use API tool_calls only; do not emit standalone Tool: lines in assistant text unless legacy text fallback is explicitly enabled for this chat.

The harness may echo executed calls as lines like:

Tool: TOOL_NAME({JSON_OBJECT})

for readability after execution; those echoes are not a substitute for native tool_calls when the API supports them.
`)
}

func LegacyToolInvocationSyntaxAppend() string {
	return strings.TrimSpace(`
Legacy text fallback is enabled for this chat: when native tool_calls are unavailable from the API, output exactly one invocation per line:

Tool: TOOL_NAME({JSON_OBJECT})

Use valid JSON objects with keys matching each tool's schema. Multiple tools: one Tool: line per tool, each on its own line.

Examples (PLAN): Tool: createPlan({"name": "feature.md", "planText": "# Goal\n\n## Steps\n1. ..."})
Examples (PLAN): Tool: editPlan({"name": "feature.md", "old": "## Steps\n1. A", "new": "## Steps\n1. B", "intent": "Reorder first step"})
Examples (BUILD): Tool: readFile({"path": "cmd/app/main.go"})
Examples (BUILD): Tool: shell({"command": "go test ./...", "intent": "Run full test suite"})
Examples (BUILD): Tool: editFile({"path": "cmd/app/main.go", "oldString": "foo", "newString": "bar", "intent": "Fix variable name"})
Examples (BUILD): Tool: searchSkill({"query": "documentation"})
Examples (BUILD): Tool: fetchWeb({"url": "https://example.com/docs"})
Examples (BUILD): Tool: webSearch({"query": "golang context cancel"})
Examples (BUILD): Tool: loadSkill({"name": "my-skill"})
`)
}

func RenderPlan(d Data) (string, error) {
	if d.Syntax == "" {
		d.Syntax = NativeToolInvocationSyntax()
	}
	return render(planRaw, d)
}

func RenderBuild(d Data) (string, error) {
	if d.Syntax == "" {
		d.Syntax = NativeToolInvocationSyntax()
	}
	return render(buildRaw, d)
}

func RenderTitle(d TitleData) (string, error) {
	return executeTemplate("title", titleRaw, d)
}

func RenderSummarize(d SummarizeData) (string, error) {
	return executeTemplate("summarize", summarizeRaw, d)
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
