package instructions

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
)

var ruleFileRe = regexp.MustCompile(`^rule_(\d+)\.txt$`)

type RuleEntry struct {
	Number  int
	Text    string
	Scope   string
	FilePath string
}

func rulesDir(scope, projHex string) (string, error) {
	switch scope {
	case ScopeGlobal:
		return paths.GlobalRulesDir()
	case ScopeProject:
		if strings.TrimSpace(projHex) == "" {
			return "", fmt.Errorf("missing project id")
		}
		return paths.ProjectRulesDir(projHex)
	default:
		return "", fmt.Errorf("unknown rules scope %q", scope)
	}
}

func ensureRulesDir(scope, projHex string) (string, error) {
	dir, err := rulesDir(scope, projHex)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func listRuleFiles(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if ruleFileRe.MatchString(e.Name()) {
			files = append(files, e.Name())
		}
	}
	sort.Slice(files, func(i, j int) bool {
		ni, _ := ruleNumberFromName(files[i])
		nj, _ := ruleNumberFromName(files[j])
		return ni < nj
	})
	return files, nil
}

func ruleNumberFromName(name string) (int, error) {
	m := ruleFileRe.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0, fmt.Errorf("invalid rule file name %q", name)
	}
	return strconv.Atoi(m[1])
}

func ruleFileName(n int) string {
	return fmt.Sprintf("rule_%02d.txt", n)
}

func AddRule(scope, projHex, text string) (n int, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "add rule failed", logging.LogOptions{Params: map[string]any{"scope": scope, "err": err.Error()}})
		}
	}()
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("rule text is empty")
	}
	dir, err := ensureRulesDir(scope, projHex)
	if err != nil {
		return 0, err
	}
	files, err := listRuleFiles(dir)
	if err != nil {
		return 0, err
	}
	next := len(files) + 1
	p := filepath.Join(dir, ruleFileName(next))
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		return 0, err
	}
	return next, nil
}

func RemoveRule(scope, projHex string, number int) (err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "remove rule failed", logging.LogOptions{Params: map[string]any{"scope": scope, "number": number, "err": err.Error()}})
		}
	}()
	if number <= 0 {
		return fmt.Errorf("invalid rule number %d", number)
	}
	dir, err := rulesDir(scope, projHex)
	if err != nil {
		return err
	}
	target := ruleFileName(number)
	p := filepath.Join(dir, target)
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("rule %d not found", number)
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	return renumberRules(dir)
}

func renumberRules(dir string) error {
	files, err := listRuleFiles(dir)
	if err != nil {
		return err
	}
	type item struct {
		num  int
		text string
	}
	var items []item
	for _, name := range files {
		n, err := ruleNumberFromName(name)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		items = append(items, item{num: n, text: string(b)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].num < items[j].num })
	for _, name := range files {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for i, it := range items {
		p := filepath.Join(dir, ruleFileName(i+1))
		if err := os.WriteFile(p, []byte(it.text), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func ListRules(scope, projHex string) (out []RuleEntry, err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "list rules failed", logging.LogOptions{Params: map[string]any{"scope": scope, "err": err.Error()}})
		}
	}()
	dir, err := rulesDir(scope, projHex)
	if err != nil {
		return nil, err
	}
	files, err := listRuleFiles(dir)
	if err != nil {
		return nil, err
	}
	out = make([]RuleEntry, 0, len(files))
	for _, name := range files {
		n, err := ruleNumberFromName(name)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		out = append(out, RuleEntry{
			Number:   n,
			Text:     strings.TrimSpace(string(b)),
			Scope:    scope,
			FilePath: filepath.Join(dir, name),
		})
	}
	return out, nil
}

func ParseRuleNumber(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("missing rule number")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid rule number %q", s)
	}
	return n, nil
}

func WriteRulesList(w io.Writer, projHex string) error {
	global, err := ListRules(ScopeGlobal, "")
	if err != nil {
		return err
	}
	project, err := ListRules(ScopeProject, projHex)
	if err != nil {
		return err
	}
	if len(global) == 0 && len(project) == 0 {
		termcolor.WriteSystem(w, "No custom rules.")
		return nil
	}
	var buf bytes.Buffer
	if len(global) > 0 {
		fmt.Fprintln(&buf, "Global rules:")
		for _, r := range global {
			fmt.Fprintf(&buf, "  %d. %s\n", r.Number, previewRule(r.Text))
		}
	}
	if len(project) > 0 {
		fmt.Fprintln(&buf, "Project rules:")
		for _, r := range project {
			fmt.Fprintf(&buf, "  %d. %s\n", r.Number, previewRule(r.Text))
		}
	}
	termcolor.WriteSystem(w, buf.String())
	return nil
}

func previewRule(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 120 {
		return s
	}
	return s[:117] + "..."
}

func LoadAllRulesText(projHex string) (string, error) {
	global, err := ListRules(ScopeGlobal, "")
	if err != nil {
		return "", err
	}
	project, err := ListRules(ScopeProject, projHex)
	if err != nil {
		return "", err
	}
	if len(global) == 0 && len(project) == 0 {
		return "", nil
	}
	var b strings.Builder
	n := 1
	for _, r := range global {
		fmt.Fprintf(&b, "%d. %s\n", n, r.Text)
		n++
	}
	for _, r := range project {
		fmt.Fprintf(&b, "%d. %s\n", n, r.Text)
		n++
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
