package compile

import (
	"regexp"
	"strconv"
	"strings"
)

const GoGuidance = "Use explicit imports for every referenced Go package and remove unused imports and variables. strings.Index returns one int (-1 when absent), not a value/boolean pair; strings.Cut returns before, after, found. Use = to reassign existing variables; := requires at least one new variable in the same scope. Go strings have no methods: use strings package functions. Discover exact SDK names, arguments, and return values with searchTools; do not infer signatures from other languages."

var compilerLocation = regexp.MustCompile(`^(?:\./)?main\.go:(\d+)(?::(\d+))?:\s*(.*)$`)

func CompilerDiagnostics(output string) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	for _, line := range strings.Split(output, "\n") {
		match := compilerLocation.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		row, _ := strconv.Atoi(match[1])
		column, _ := strconv.Atoi(match[2])
		category, hint := compilerHint(match[3])
		diagnostics = append(diagnostics, SourceDiagnostic{Line: row, Column: column, Message: match[3], Category: category, Hint: hint})
		if len(diagnostics) == 8 {
			break
		}
	}
	return diagnostics
}

func compilerHint(message string) (string, string) {
	switch {
	case strings.Contains(message, "no new variables on left side of :="):
		return "declaration", "Use = to reassign existing variables, or introduce a genuinely new variable; := requires a new variable in this scope."
	case strings.Contains(message, "strings.Index") && strings.Contains(message, "assignment mismatch"):
		return "return_values", "strings.Index returns one int; assign index := strings.Index(text, needle), then test index >= 0. Use strings.Cut for before, after, found."
	case strings.Contains(message, "imported and not used"):
		return "import", "Remove this unused import or use the imported package; Go rejects unused imports."
	case strings.HasPrefix(message, "undefined: sdk.") || strings.Contains(message, "sdk.") && (strings.Contains(message, "arguments") || strings.Contains(message, "assignment mismatch")):
		return "sdk_signature", "Use searchTools to retrieve the exact SDK function name, argument types, and return values before correcting this call."
	case strings.HasPrefix(message, "undefined:"):
		return "undefined_symbol", "Check the identifier and its scope. If it names a standard-library package, add its explicit import; discover SDK helpers with searchTools."
	case strings.Contains(message, "assignment mismatch") || strings.Contains(message, "multiple-value") || strings.Contains(message, "not enough arguments") || strings.Contains(message, "too many arguments"):
		return "signature", "Match the function's exact parameter and return counts and types; handle returned errors explicitly."
	case strings.Contains(message, "declared and not used"):
		return "unused_variable", "Use or remove this variable; use _ only for a return value you intentionally discard."
	case strings.Contains(message, "has no field or method"):
		return "member", "Go strings have no methods; use functions from strings. For SDK values, check the exact returned type and available fields with searchTools."
	default:
		return "type", "Correct the reported Go type or declaration before retrying; do not repeat unchanged source."
	}
}
