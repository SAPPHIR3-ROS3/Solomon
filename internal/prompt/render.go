package prompt

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed templates/plan.tmpl
var planRaw string

//go:embed templates/build.tmpl
var buildRaw string

type Data struct {
	Tools      string
	Syntax     string
	ExtraRules string
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

func render(raw string, d Data) (string, error) {
	t, err := template.New("p").Parse(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, d); err != nil {
		return "", err
	}
	return b.String(), nil
}
