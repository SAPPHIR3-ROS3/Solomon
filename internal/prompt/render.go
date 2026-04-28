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
Tool: TOOL_NAME({JSON_OBJECT})

Rules:
- Exactly one tool invocation per line; each line must match this pattern.
- Immediately before each line starting with "Tool: ", output two newline characters after any preceding assistant text or after a previous tool line (one blank line; in plain text this is the sequence \n\n before each tool call).
- TOOL_NAME must be one of the tools listed under "Available tools" below.
- The parentheses contain a single JSON object whose keys match the JSON field names accepted by that tool (see each tool's signature and description).
- JSON must be valid UTF-8 and compact (minimal whitespace, single line).
- Multiple tools: insert \n\n before every Tool: line, including before the second and later tools.

Examples (PLAN mode tools; each Tool: line is preceded by a blank line as required):

Tool: createPlan({"name": "feature.md", "planText": "# Goal\n\n## Steps\n1. ..."})

Tool: editPlan({"name": "feature.md", "old": "## Steps\n1. old", "new": "## Steps\n1. new"})

Tool: buildPlan({"name": "feature.md"})

Examples (BUILD mode tools; each Tool: line is preceded by a blank line as required):

Tool: readFile({"path": "cmd/app/main.go"})

Tool: shell({"command": "go test ./..."})

Tool: editFile({"path": "README.md", "oldString": "foo", "newString": "bar"})

Tool: subagent({"sysPromptPath": ".solomon/subagent.md", "task": "Summarize pkg/foo"})
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
