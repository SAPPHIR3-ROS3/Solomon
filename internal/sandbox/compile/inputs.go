package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const InputGuidance = "Keep payloads separate from Go source: put regex patterns, replacement text, shell commands, and file content containing backslashes, embedded quotes, backticks, or newlines in orchestrate.inputs (a JSON object of string values). Read each value with sdk.Input(name) (string, error), checking its error. Input values are plain text, JSON-escaped once; never pre-escape them as Go literals. Solomon quotes them for Go without changing their contents. For existing file content, prefer sdk.ReadFile and transform in Go. Do not repair escaping by switching to another interpreter."

func writeInputs(dir string, inputs map[string]string) error {
	if len(inputs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var source strings.Builder
	fmt.Fprintf(&source, "package main\nimport sdk %s\nfunc init() { sdk.SetInputs(map[string]string{\n", strconv.Quote(SDKImportCanonical))
	for _, key := range keys {
		fmt.Fprintf(&source, "%s: %s,\n", strconv.Quote(key), strconv.Quote(inputs[key]))
	}
	source.WriteString("}) }\n")
	return os.WriteFile(filepath.Join(dir, "inputs.go"), []byte(source.String()), 0o600)
}
