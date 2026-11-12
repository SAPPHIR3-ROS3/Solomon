package compile

import (
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

const SDKImportCanonical = "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"

var SDKImportPathsForModel = []string{
	"sdk",
}

var sdkImportAliasPaths = map[string]struct{}{
	"sdk": {},
}

func RewriteSDKImports(src string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		return src, err
	}
	type span struct{ start, end int }
	var reps []span
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !isSDKImportAlias(path) {
			continue
		}
		start := fset.Position(imp.Path.Pos()).Offset + 1
		end := fset.Position(imp.Path.End()).Offset - 1
		reps = append(reps, span{start: start, end: end})
	}
	if len(reps) == 0 {
		return src, nil
	}
	b := []byte(src)
	for i := len(reps) - 1; i >= 0; i-- {
		r := reps[i]
		if r.start < 0 || r.end > len(b) || r.start >= r.end {
			continue
		}
		b = append(append(append([]byte(nil), b[:r.start]...), SDKImportCanonical...), b[r.end:]...)
	}
	return string(b), nil
}

func SourceReferencesSDKAlias(src string) bool {
	lower := strings.ToLower(src)
	for path := range sdkImportAliasPaths {
		if strings.Contains(lower, `"`+strings.ToLower(path)+`"`) {
			return true
		}
	}
	return false
}

func orchestrateParseError(parseErr error, src string) string {
	if parseErr == nil {
		return ""
	}
	msg := fmt.Sprintf("invalid Go source: %v", parseErr)
	if list, ok := parseErr.(scanner.ErrorList); ok && len(list) > 0 {
		first := list[0]
		offset := first.Pos.Offset
		if offset >= 0 && offset <= len(src) {
			left, right := offset, offset
			for left > 0 && offset-left < 60 && src[left-1] != '\n' {
				left--
			}
			for right < len(src) && right-offset < 60 && src[right] != '\n' {
				right++
			}
			msg += fmt.Sprintf("\nSource near main.go:%d:%d (quoted): %q", first.Pos.Line, first.Pos.Column, src[left:right])
		}
		switch {
		case strings.Contains(first.Msg, "unknown escape sequence"):
			msg += "\nHint: invalid escape in a Go quoted literal. For a literal backslash, double it in Go source (for example, \"\\\\d+\", not \"\\d+\"). A raw string preserves backslashes but cannot contain backticks. JSON escaping is a separate layer."
		case strings.Contains(first.Msg, "raw string literal not terminated"):
			msg += "\nHint: close the raw string with a backtick. Raw strings cannot contain backticks; read existing text with sdk.ReadFile instead of embedding it."
		case strings.Contains(first.Msg, "string literal not terminated"):
			msg += "\nHint: close the double-quoted Go string; escape embedded double quotes and encode newlines as Go escapes."
		case strings.Contains(first.Msg, "missing ','"):
			msg += "\nHint: check the separator and string delimiters at this position and immediately before it; multiline composite literals require a trailing comma."
		case strings.Contains(first.Msg, "illegal character"):
			msg += "\nHint: this character is not valid Go syntax at this position. Check whether shell syntax or embedded text escaped its string literal; keep such text inside a valid Go string or read it through the SDK."
		case strings.Contains(first.Msg, "expected declaration"):
			msg += "\nHint: only declarations are allowed outside functions. Check for an extra closing brace or a function closed too early; executable statements belong inside func main()."
		case strings.Contains(first.Msg, "expected ')'") || strings.Contains(first.Msg, "expected ']'") || strings.Contains(first.Msg, "expected '}'"):
			msg += "\nHint: check matching parentheses, brackets, and braces before this position. The missing or mismatched delimiter may be on an earlier line; fix the first syntax error before subsequent errors."
		case strings.Contains(first.Msg, "expected operand"):
			msg += "\nHint: an expression is required here. Check for a missing value, an extra closing delimiter, or a malformed function argument immediately before this position."
		case strings.Contains(first.Msg, "expected ';'"):
			msg += "\nHint: check the preceding statement and its string or expression delimiters. Put separate Go statements on separate lines rather than joining them into one dense line."
		}
	}
	if SourceReferencesSDKAlias(src) {
		msg += "\nSDK import rewriting requires valid Go syntax; fix the reported syntax error first."
	}
	return msg
}

func clarifyCompileError(src, msg string) string {
	if strings.Contains(msg, "package sdk is not in std") && SourceReferencesSDKAlias(src) {
		return msg + " (orchestrate: import \"sdk\" was not rewritten — fix Go syntax first, then retry)"
	}
	return msg
}

func isSDKImportAlias(path string) bool {
	_, ok := sdkImportAliasPaths[path]
	return ok
}
