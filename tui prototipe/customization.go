package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/live"
	tea "github.com/charmbracelet/bubbletea"
)

var activeInterface = live.Default()
var interfaceRevision uint64

type interfaceMsg struct {
	changed   bool
	cfg       live.Config
	err       error
	request   string
	devStatus string
}

func (m model) pollInterface() tea.Cmd {
	watcher := m.watcher
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		result := interfaceMsg{}
		if watcher != nil {
			result.changed, result.err = watcher.Poll()
			if result.changed {
				result.cfg = watcher.Current
			}
		}
		if path := os.Getenv("SOLOMON_TUI_RELOAD_REQUEST"); path != "" {
			if data, err := os.ReadFile(path); err == nil {
				var request struct{ Token string }
				if json.Unmarshal(data, &request) == nil {
					result.request = request.Token
				}
			}
		}
		if path := os.Getenv("SOLOMON_TUI_DEV_STATUS"); path != "" {
			if data, err := os.ReadFile(path); err == nil {
				var status struct{ Phase, Message string }
				if json.Unmarshal(data, &status) == nil && (strings.Contains(status.Phase, "error") || status.Phase == "failed") {
					result.devStatus = strings.ReplaceAll(status.Message, "\n", " · ")
				}
			}
		}
		return result
	})
}
func applyInterface(cfg live.Config) {
	activeInterface = cfg
	interfaceRevision++
	imageRows = map[int]string{}
	border = cfg.Colors["border"]
	ink = cfg.Colors["text"]
	muted = cfg.Colors["muted"]
	gold = cfg.Colors["gold"]
	cyan = cfg.Colors["user"]
	green = cfg.Colors["assistant"]
	violet = cfg.Colors["system"]
}
func configuredText(name string) string            { return activeInterface.Text[name] }
func configuredHint(context, action string) string { return activeInterface.Hint(context, action) }
func interfaceArgs(args []string) ([]string, error) {
	filtered := []string{args[0]}
	for i := 1; i < len(args); i++ {
		if args[i] == "--config" {
			if i+1 == len(args) {
				return nil, fmt.Errorf("--config requires a path")
			}
			i++
			if err := os.Setenv("SOLOMON_TUI_CONFIG", args[i]); err != nil {
				return nil, err
			}
			continue
		}
		filtered = append(filtered, args[i])
	}
	return filtered, nil
}
func inputHint() string {
	return configuredHint("composer", "send") + " send  ·  " + configuredHint("composer", "newline") + " newline"
}
func cleanNotice(err error) string {
	if err == nil {
		return ""
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

func interfaceBorder() (string, string, string, string, string, string) {
	switch activeInterface.Layout.Border {
	case "square":
		return "┌", "┐", "└", "┘", "─", "│"
	case "heavy":
		return "┏", "┓", "┗", "┛", "━", "┃"
	default:
		return "╭", "╮", "╰", "╯", "─", "│"
	}
}
func (m model) contentWidth() int { return max(1, m.width-2*activeInterface.Layout.Margin) }
func keyCaption(key string) string {
	switch key {
	case "tab":
		return "Tab"
	case "enter":
		return "Enter"
	case "esc":
		return "Esc"
	case "up":
		return "↑"
	case "down":
		return "↓"
	}
	return key
}
