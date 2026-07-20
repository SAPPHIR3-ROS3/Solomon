package shellhist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt/shellutils"
)

func Suggest(prefix string) string {
	if prefix == "" {
		return ""
	}
	path, kind := resolveHistoryFile()
	if path == "" {
		return ""
	}
	lines, err := readHistoryLines(path)
	if err != nil {
		return ""
	}
	for i := len(lines) - 1; i >= 0; i-- {
		cmd := parseHistoryLine(lines[i], kind)
		if cmd == "" || cmd == prefix {
			continue
		}
		if strings.HasPrefix(cmd, prefix) {
			return cmd
		}
	}
	return ""
}

func Append(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	path, kind := resolveHistoryFile()
	if path == "" || kind == historyNone {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := formatAppendLine(command, kind, path)
	if line == "" {
		return nil
	}
	_, err = f.WriteString(line)
	return err
}

type historyKind int

const (
	historyNone historyKind = iota
	historyPlain
	historyZshExtended
	historyFish
)

func resolveHistoryFile() (string, historyKind) {
	sh := strings.ToLower(shellutils.Effective())
	if runtime.GOOS != "windows" && strings.Contains(sh, "fish") {
		dir, _ := os.UserHomeDir()
		if dir == "" {
			return "", historyNone
		}
		return filepath.Join(dir, ".local", "share", "fish", "fish_history"), historyFish
	}
	if p := strings.TrimSpace(os.Getenv("HISTFILE")); p != "" {
		kind := historyPlain
		if strings.Contains(sh, "zsh") {
			kind = detectZshFormat(p)
		}
		return p, kind
	}
	if runtime.GOOS == "windows" {
		return psReadLinePath()
	}
	dir, _ := os.UserHomeDir()
	if dir == "" {
		return "", historyNone
	}
	if strings.Contains(sh, "zsh") {
		p := filepath.Join(dir, ".zsh_history")
		return p, detectZshFormat(p)
	}
	return filepath.Join(dir, ".bash_history"), historyPlain
}

func detectZshFormat(path string) historyKind {
	f, err := os.Open(path)
	if err != nil {
		return historyZshExtended
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ": ") && strings.Contains(line, ";") {
			return historyZshExtended
		}
		return historyPlain
	}
	return historyZshExtended
}

func readHistoryLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func parseHistoryLine(line string, kind historyKind) string {
	switch kind {
	case historyZshExtended:
		return parseZshExtended(line)
	case historyFish:
		return parseFishLine(line)
	default:
		return strings.TrimSpace(line)
	}
}

func parseZshExtended(line string) string {
	if !strings.HasPrefix(line, ": ") {
		return strings.TrimSpace(line)
	}
	idx := strings.LastIndex(line, ";")
	if idx < 0 {
		return ""
	}
	return line[idx+1:]
}

func parseFishLine(line string) string {
	if !strings.HasPrefix(line, "- cmd: ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "- cmd: "))
}

func formatAppendLine(command string, kind historyKind, path string) string {
	switch kind {
	case historyZshExtended:
		return fmt.Sprintf(": %d:0;%s\n", time.Now().Unix(), command)
	case historyFish:
		return "- cmd: " + command + "\n"
	case historyPlain:
		return command + "\n"
	default:
		return ""
	}
}
