package server

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"strings"
)

const maxGoFormatBody = 1 << 20

func handleFormatGo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGoFormatBody))
	if err != nil {
		status := http.StatusBadRequest
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			status = http.StatusRequestEntityTooLarge
		}
		writeAPIError(w, status, err)
		return
	}
	var request struct {
		Source *string `json:"source"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("invalid JSON request"))
		return
	}
	if request.Source == nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("source must be a string"))
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Source string `json:"source"`
	}{Source: formatGoDisplay(*request.Source)})
}

// formatGoDisplay accepts files, declaration fragments, and statement fragments.
// Parsing must succeed before any normalization; partial ASTs are never used.
func formatGoDisplay(source string) string {
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
		// Only syntactic boundaries are changed, never token contents. In
		// particular, init/condition/post statements are not block-list entries.
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
			if breaks[offset] && offset > 0 && source[offset-1] != '\n' && (offset == len(source) || source[offset] != '\n') {
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
		return string(formatted)
	}
	return source
}
