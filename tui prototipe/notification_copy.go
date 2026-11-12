package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type notificationCopyResult struct{ err error }

func (m model) copyNotification(key string) tea.Cmd {
	var entry message
	if m.componentOpen {
		if componentNames[m.componentIndex%len(componentNames)] != "Notification" {
			return nil
		}
		for _, candidate := range galleryMessages() {
			if candidate.role == "NOTICE" {
				entry = candidate
				break
			}
		}
	} else {
		fields := strings.Split(key, ".")
		if len(fields) != 2 || fields[0] != "chat" {
			return nil
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil || index < 0 || index >= len(m.messages) || m.messages[index].role != "NOTICE" {
			return nil
		}
		entry = m.messages[index]
	}
	if entry.role != "NOTICE" {
		return nil
	}
	_, content, found := strings.Cut(entry.text, "\n")
	if !found {
		content = entry.text
	}
	return func() tea.Msg {
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		_, err := fmt.Fprintf(os.Stderr, "\x1b]52;c;%s\a", encoded)
		return notificationCopyResult{err: err}
	}
}
