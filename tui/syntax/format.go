package syntax

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
)

func FormatGo(source string) string {
	for _, wrapper := range [][2]string{
		{"", ""},
		{"package display\n", ""},
		{"package display\nfunc _() {\n", "\n}"},
	} {
		prefix, suffix := wrapper[0], wrapper[1]
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "", prefix+source+suffix, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		breaks := make([]bool, len(source)+1)
		mark := func(pos token.Pos) {
			offset := fset.PositionFor(pos, false).Offset - len(prefix)
			if offset >= 0 && offset <= len(source) {
				breaks[offset] = true
			}
		}
		statements := func(list []ast.Stmt) {
			for _, stmt := range list {
				mark(stmt.Pos())
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.BlockStmt:
				if len(node.List) > 0 {
					mark(node.Lbrace + 1)
					statements(node.List)
					mark(node.Rbrace)
				}
			case *ast.CaseClause:
				statements(node.Body)
			case *ast.CommClause:
				statements(node.Body)
			}
			return true
		})
		var normalized strings.Builder
		for offset := 0; offset <= len(source); offset++ {
			if breaks[offset] && offset > 0 && needsDisplayBreak(source, offset) && (offset == len(source) || source[offset] != '\n') {
				normalized.WriteByte('\n')
			}
			if offset < len(source) {
				normalized.WriteByte(source[offset])
			}
		}
		formatted, err := format.Source([]byte(normalized.String()))
		if err != nil {
			return source
		}
		return twoSpaceIndent(string(formatted))
	}
	return source
}

func twoSpaceIndent(source string) string {
	set := token.NewFileSet()
	file := set.AddFile("display.go", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, []byte(source), func(token.Position, string) {}, scanner.ScanComments)
	type span struct{ start, end int }
	protected := []span{}
	for {
		pos, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.STRING || kind == token.CHAR || kind == token.COMMENT {
			start := file.Offset(pos)
			end := start + len(literal)
			if source[start] == '`' {
				if last := strings.IndexByte(source[start+1:], '`'); last >= 0 {
					end = start + last + 2
				}
			}
			protected = append(protected, span{start, end})
		}
	}
	var result strings.Builder
	offset, protectedIndex := 0, 0
	for _, line := range strings.SplitAfter(source, "\n") {
		for protectedIndex < len(protected) && protected[protectedIndex].end <= offset {
			protectedIndex++
		}
		inside := protectedIndex < len(protected) && protected[protectedIndex].start < offset && offset < protected[protectedIndex].end
		if inside {
			result.WriteString(line)
		} else {
			tabs := 0
			for tabs < len(line) && line[tabs] == '\t' {
				tabs++
			}
			result.WriteString(strings.Repeat(" ", Display.IndentSize*tabs))
			result.WriteString(line[tabs:])
		}
		offset += len(line)
	}
	return result.String()
}

func needsDisplayBreak(source string, offset int) bool {
	start := strings.LastIndexByte(source[:offset], '\n') + 1
	for _, character := range source[start:offset] {
		if character != ' ' && character != '\t' && character != '\r' {
			return true
		}
	}
	return false
}
