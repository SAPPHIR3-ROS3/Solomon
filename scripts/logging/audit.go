package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

var patterns = map[string]*regexp.Regexp{
	"error_handling":  regexp.MustCompile("\\bif\\s+(?:\\w+\\s*:=.*;\\s*)?err\\s*!=\\s*nil|\\bcatch\\b|\\bexcept\\b"),
	"logging":         regexp.MustCompile("\\blogging\\.|\\blog\\.(?:Print|Fatal)|\\bconsole\\.(?:error|warn|log)|\\bslog\\."),
	"processes":       regexp.MustCompile("exec\\.Command(?:Context)?\\(|\\b(?:spawn|execFile|subprocess\\.run)\\("),
	"ignored_results": regexp.MustCompile("_\\s*=\\s*.*\\.(?:Write|Save|Encode|Run|Wait|Close|Remove|Rename)\\w*\\("),
	"silent_catch":    regexp.MustCompile("catch\\s*\\([^)]*\\)\\s*\\{\\s*\\}|catch\\s*\\{\\s*\\}|\\.catch\\(\\(\\)\\s*=>\\s*\\{\\s*\\}\\)"),
}

type candidate struct {
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

type report struct {
	FilesScanned     int                       `json:"files_scanned"`
	Scope            string                    `json:"scope"`
	Interpretation   string                    `json:"interpretation"`
	Groups           map[string]map[string]int `json:"groups"`
	ReviewCandidates []candidate               `json:"review_candidates"`
}

func audit(root string) (report, error) {
	r := report{Scope: "repository-owned runtime code and scripts, excluding tests and generated or bundled assets", Interpretation: "Heuristic review inventory, not proof of a defect. Errors may be observed by callers, HTTP middleware, tool dispatch, or process helpers. Expected probes and cleanup may intentionally remain quiet.", Groups: make(map[string]map[string]int), ReviewCandidates: []candidate{}}
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return r, err
	}
	names := strings.Split(string(out), "\x00")
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		if name == "" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		switch filepath.Ext(name) {
		case ".go", ".ts", ".tsx", ".js", ".mjs", ".py", ".sh", ".ps1":
		default:
			continue
		}
		parts := strings.Split(name, "/")
		if slices.ContainsFunc(parts, func(part string) bool {
			switch part {
			case "node_modules", "vendor", "dist", "build", "assets", "wailsjs", ".git", ".venv", "__pycache__", "test", "tests", "e2e":
				return true
			}
			return false
		}) {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return r, err
		}
		text := strings.ReplaceAll(strings.ReplaceAll(strings.ToValidUTF8(string(body), "�"), "\r\n", "\n"), "\r", "\n")
		if prefix := []rune(text); strings.Contains(string(prefix[:min(1000, len(prefix))]), "Code generated") {
			continue
		}
		r.FilesScanned++
		group := parts[0]
		if len(parts) > 2 {
			group += "/" + parts[1]
		}
		counts := r.Groups[group]
		if counts == nil {
			counts = make(map[string]int)
			r.Groups[group] = counts
		}
		counts["files"]++
		flags := map[string]bool{}
		for key, pattern := range patterns {
			if pattern.MatchString(text) {
				counts[key]++
				flags[key] = true
			}
		}
		var reasons []string
		if !flags["logging"] && flags["processes"] {
			reasons = append(reasons, "process_without_local_logging")
		}
		if flags["ignored_results"] {
			reasons = append(reasons, "ignored_result_requires_review")
		}
		if flags["silent_catch"] {
			reasons = append(reasons, "silent_catch_requires_review")
		}
		if len(reasons) > 0 {
			r.ReviewCandidates = append(r.ReviewCandidates, candidate{Path: name, Reasons: reasons})
		}
	}
	return r, nil
}

func run() error {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..")
	if !filepath.IsAbs(root) {
		root = "."
	}
	r, err := audit(root)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(r)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
