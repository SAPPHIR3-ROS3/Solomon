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

type Data struct {
	Tools                 string
	Syntax                string
	ExtraRules            string
	Language              string
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

func ToolInvocationSyntax() string {
	return strings.TrimSpace(`
Native function calling: use the API tool/functions exposed for this session (names and JSON schemas match the tools below). Do not rely on embedding "Tool: ..." lines in assistant text unless the provider does not return tool calls.

The harness may echo executed calls as lines like:

Tool: TOOL_NAME({JSON_OBJECT})

for readability; that echo is not a substitute for real tool_calls when the API supports them.

Legacy fallback (text only): if tool_calls are unavailable, output exactly one invocation per line:

Tool: TOOL_NAME({JSON_OBJECT})

Use valid JSON objects with keys matching each tool's schema. Multiple tools: one Tool: line per tool, each on its own line.

Examples (PLAN): Tool: createPlan({"name": "feature.md", "planText": "# Goal\n\n## Steps\n1. ..."})
Examples (BUILD): Tool: readFile({"path": "cmd/app/main.go"}), Tool: shell({"command": "go test ./..."})
`)
}

func RenderPlan(d Data) (string, error) {
	if d.Syntax == "" {
		d.Syntax = ToolInvocationSyntax()
	}
	return render(planRaw, d)
}

func RenderBuild(d Data) (string, error) {
	if d.Syntax == "" {
		d.Syntax = ToolInvocationSyntax()
	}
	return render(buildRaw, d)
}

func RenderTitle(d TitleData) (string, error) {
	return executeTemplate("title", titleRaw, d)
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
	for _, key := range []string{"SHELL", "COMSPEC"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if runtime.GOOS == "windows" {
		if p := windowsFallbackShellExecutable(); p != "" {
			return p
		}
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
