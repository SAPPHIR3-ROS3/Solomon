package compile

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// MCPToolBinding describes the names made available to an orchestrate script.
// The source-level form is sdk.mcp.<tool>(intent, args); it is rewritten to a
// normal sandbox RPC call before the script is compiled.
type MCPToolBinding struct {
	ExposedName string
	ServerName  string
	ToolName    string
}

// MCPFunctionName is the unqualified source identifier for an MCP tool. It is
// usable when the tool name is unique across connected servers.
func MCPFunctionName(toolName string) string {
	return sanitizeMCPIdentifier(toolName, "tool")
}

// MCPQualifiedFunctionName is the collision-free source identifier for an MCP
// tool, combining its server and tool names.
func MCPQualifiedFunctionName(serverName, toolName string) string {
	return sanitizeMCPIdentifier(serverName+"_"+toolName, "tool")
}

// RewriteMCPCalls lowers the intentionally small Code Mode MCP syntax into
// the existing sandbox RPC surface. This keeps MCP on the same orchestrate
// path as every other deferred tool while allowing the model to write the
// established sdk.mcp.<tool>(intent, args) form.
func RewriteMCPCalls(src string, bindings []MCPToolBinding) (string, error) {
	if len(bindings) == 0 || !strings.Contains(src, "sdk.mcp.") {
		return src, nil
	}

	byName := make(map[string]string, len(bindings)*3)
	ambiguous := make(map[string]bool)
	for _, binding := range bindings {
		exposed := strings.TrimSpace(binding.ExposedName)
		if exposed == "" {
			exposed = "MCP." + strings.TrimSpace(binding.ServerName) + "." + strings.TrimSpace(binding.ToolName)
		}
		qualified := MCPQualifiedFunctionName(binding.ServerName, binding.ToolName)
		byName[qualified] = exposed
		byName[sanitizeMCPIdentifier(exposed, "tool")] = exposed
		short := MCPFunctionName(binding.ToolName)
		if previous, ok := byName[short]; ok && previous != exposed {
			delete(byName, short)
			ambiguous[short] = true
		} else if !ambiguous[short] {
			byName[short] = exposed
		}
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return src, err
	}
	type replacement struct {
		start int
		end   int
		text  string
	}
	var replacements []replacement
	ast.Inspect(f, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		outer, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || outer.Sel == nil {
			return true
		}
		middle, ok := outer.X.(*ast.SelectorExpr)
		if !ok || middle.Sel == nil {
			return true
		}
		pkg, ok := middle.X.(*ast.Ident)
		if !ok || pkg.Name != "sdk" || middle.Sel.Name != "mcp" {
			return true
		}
		exposed, ok := byName[outer.Sel.Name]
		if !ok {
			return true
		}
		start := fset.Position(call.Fun.Pos()).Offset
		end := fset.Position(call.Lparen).Offset + 1
		if start < 0 || end < start || end > len(src) {
			return true
		}
		replacements = append(replacements, replacement{
			start: start,
			end:   end,
			text:  fmt.Sprintf("sdk.MCPCall(%s, ", strconv.Quote(exposed)),
		})
		return true
	})
	if len(replacements) == 0 {
		return src, nil
	}
	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start > replacements[j].start
	})
	for _, r := range replacements {
		src = src[:r.start] + r.text + src[r.end:]
	}
	return src, nil
}

func sanitizeMCPIdentifier(value, fallback string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		return fallback
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	if token.IsKeyword(name) {
		return "_" + name
	}
	return name
}
