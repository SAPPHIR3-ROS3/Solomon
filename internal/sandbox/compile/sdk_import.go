package compile

import (
	"go/parser"
	"go/token"
	"strconv"
)

const SDKImportCanonical = "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/sdk"

var SDKImportPathsForModel = []string{
	"sdk",
	"SAPPHIR3ROS3/Solomon/sdk",
	"SAPPHIR3ROS3/Solomon/v2026/sdk",
}

var sdkImportAliasPaths = map[string]struct{}{
	"sdk": {},
	"SAPPHIR3ROS3/Solomon/sdk":                   {},
	"SAPPHIR3ROS3/Solomon/v2026/sdk":             {},
	"github.com/SAPPHIR3ROS3/Solomon/sdk":          {},
	"github.com/SAPPHIR3ROS3/Solomon/v2026/sdk":    {},
	"SAPPHIR3-ROS3/Solomon/sdk":                  {},
	"SAPPHIR3-ROS3/Solomon/v2026/sdk":            {},
	"github.com/SAPPHIR3-ROS3/Solomon/sdk":       {},
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/sdk": {},
}

func RewriteSDKImports(src string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		return src
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
		return src
	}
	b := []byte(src)
	for i := len(reps) - 1; i >= 0; i-- {
		r := reps[i]
		if r.start < 0 || r.end > len(b) || r.start >= r.end {
			continue
		}
		b = append(append(append([]byte(nil), b[:r.start]...), SDKImportCanonical...), b[r.end:]...)
	}
	return string(b)
}

func isSDKImportAlias(path string) bool {
	_, ok := sdkImportAliasPaths[path]
	return ok
}
