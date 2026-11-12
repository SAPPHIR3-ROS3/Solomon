package main

import (
	sharedsyntax "github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/syntax"
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

type goHighlightToken struct {
	start, end int
	kind       token.Token
}

func highlightGo(source string) []string {
	file := token.NewFileSet().AddFile("snippet.go", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, []byte(source), func(token.Position, string) {}, scanner.ScanComments)
	var tokens []goHighlightToken
	for {
		pos, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.SEMICOLON && literal == "\n" {
			continue
		}
		start := file.Offset(pos)
		size := len(literal)
		if size == 0 {
			size = len(kind.String())
		}
		if kind == token.ILLEGAL {
			_, size = utf8.DecodeRuneInString(source[start:])
		}
		end := min(len(source), start+size)
		if kind == token.COMMENT || kind == token.STRING {
			if strings.HasPrefix(source[start:], "/*") {
				if n := strings.Index(source[start+2:], "*/"); n >= 0 {
					end = start + 2 + n + 2
				} else {
					end = len(source)
				}
			} else if source[start] == 96 {
				if n := strings.IndexByte(source[start+1:], 96); n >= 0 {
					end = start + 1 + n + 1
				} else {
					end = len(source)
				}
			} else if kind == token.COMMENT {
				if n := strings.IndexByte(source[start:], '\n'); n >= 0 {
					end = start + n
				} else {
					end = len(source)
				}
			}
		}
		tokens = append(tokens, goHighlightToken{start, end, kind})
	}
	var result strings.Builder
	cursor := 0
	brackets := []byte{}
	for i, item := range tokens {
		if item.start < cursor {
			continue
		}
		result.WriteString(source[cursor:item.start])
		color := sharedsyntax.Color(goScope(tokens, i, source))
		if bracket, opening, ok := bracketToken(item.kind); ok {
			if opening {
				color = sharedsyntax.Bracket(len(brackets))
				brackets = append(brackets, bracket)
			} else if len(brackets) > 0 && brackets[len(brackets)-1] == bracket {
				brackets = brackets[:len(brackets)-1]
				color = sharedsyntax.Bracket(len(brackets))
			}
		}
		parts := strings.Split(source[item.start:item.end], "\n")
		for j, part := range parts {
			if j > 0 {
				result.WriteByte('\n')
			}
			result.WriteString(highlightExact(part, color))
		}
		cursor = item.end
	}
	result.WriteString(source[cursor:])
	return strings.Split(result.String(), "\n")
}

func highlightExact(s, color string) string {
	if s == "" {
		return s
	}
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	if start == end {
		return s
	}
	var styled strings.Builder
	segment := start
	for i := start; i < end; i++ {
		if s[i] == '\t' || s[i] == '\r' {
			styled.WriteString(sharedsyntax.ANSI(s[segment:i], color))
			styled.WriteByte(s[i])
			segment = i + 1
		}
	}
	styled.WriteString(sharedsyntax.ANSI(s[segment:end], color))
	return s[:start] + styled.String() + s[end:]
}

func highlightedWrappedLine(row string, width int) []string {
	width = max(1, width)
	row = expandDisplayTabs(row)
	return sharedsyntax.RestoreWrappedStyles(strings.Split(ansi.Hardwrap(row, width, true), "\n"))
}

func goScope(tokens []goHighlightToken, index int, source string) string {
	item := tokens[index]
	switch {
	case item.kind.IsKeyword():
		return "keyword.control.go"
	case item.kind == token.STRING || item.kind == token.CHAR:
		return "string.quoted.double.go"
	case item.kind == token.INT || item.kind == token.FLOAT || item.kind == token.IMAG:
		return "constant.numeric.decimal.go"
	case item.kind == token.COMMENT:
		return "comment.line.double-slash.go"
	case item.kind == token.IDENT:
		name := source[item.start:item.end]
		if index > 0 && tokens[index-1].kind == token.PACKAGE {
			return "entity.name.type.package.go"
		}
		for _, builtin := range []string{"bool", "byte", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr"} {
			if name == builtin {
				return "storage.type.go"
			}
		}
		next := index + 1
		for next < len(tokens) && tokens[next].kind == token.COMMENT {
			next++
		}
		if next < len(tokens) && tokens[next].kind == token.LPAREN {
			return "entity.name.function.go"
		}
		if index > 0 && tokens[index-1].kind == token.PERIOD {
			return "variable.other.property.go"
		}
		return "variable.other.go"
	case item.kind == token.ASSIGN || item.kind == token.DEFINE || item.kind >= token.ADD_ASSIGN && item.kind <= token.AND_NOT_ASSIGN:
		return "keyword.operator.assignment.go"
	case item.kind == token.ADD || item.kind == token.SUB || item.kind == token.MUL || item.kind == token.QUO || item.kind == token.REM || item.kind == token.AND || item.kind == token.OR || item.kind == token.XOR || item.kind == token.SHL || item.kind == token.SHR || item.kind == token.AND_NOT:
		return "keyword.operator.arithmetic.go"
	case item.kind.IsOperator():
		return "keyword.operator.go"
	default:
		return "variable.other.go"
	}
}
func bracketToken(kind token.Token) (byte, bool, bool) {
	switch kind {
	case token.LPAREN:
		return '(', true, true
	case token.LBRACK:
		return '[', true, true
	case token.LBRACE:
		return '{', true, true
	case token.RPAREN:
		return '(', false, true
	case token.RBRACK:
		return '[', false, true
	case token.RBRACE:
		return '{', false, true
	}
	return 0, false, false
}

func highlightedCodeLine(row string, width int) []string {
	width = max(1, width)
	row = expandDisplayTabs(row)
	plain := ansi.Strip(row)
	indentation := len(plain) - len(strings.TrimLeft(plain, " "))
	indentation = min(indentation, max(0, width-1))
	content := ansi.TruncateLeft(row, indentation, "")
	parts := highlightedWrappedLine(content, max(1, width-indentation))
	prefix := strings.Repeat(" ", indentation)
	for i := range parts {
		parts[i] = prefix + parts[i]
	}
	return parts
}

func expandDisplayTabs(row string) string {
	parts := strings.Split(row, "\t")
	if len(parts) == 1 {
		return row
	}
	var result strings.Builder
	column := 0
	for index, part := range parts {
		if index > 0 {
			spaces := sharedsyntax.Display.IndentSize - column%sharedsyntax.Display.IndentSize
			result.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		}
		result.WriteString(part)
		column += ansi.StringWidth(part)
	}
	return result.String()
}
