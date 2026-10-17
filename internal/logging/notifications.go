package logging

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Notification struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

var notificationMemory []Notification

func notification(date, level, message string) Notification {
	sum := sha256.Sum256([]byte(date + "\x00" + level + "\x00" + message))
	return Notification{ID: fmt.Sprintf("%x", sum), CreatedAt: date, Level: level, Message: message}
}

// RecordNotification keeps system output and GUI errors independently of chat transcripts.
func RecordNotification(level, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	recordNotificationLocked(notification(time.Now().UTC().Format(time.RFC3339Nano), level, message))
}

func recordNotificationLocked(entry Notification) {
	notificationMemory = append(notificationMemory, entry)
	if globalLogDir == "" {
		return
	}
	if err := os.MkdirAll(globalLogDir, 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(globalLogDir, "notifications.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewEncoder(file).Encode(entry)
}

var notificationLogPattern = regexp.MustCompile(`^\[([^]]+)\]\[(ERROR|WARNING|SYSTEM)\]\[[^]]*\]\[[^]]*\] (.*)$`)
var logEntryPattern = regexp.MustCompile(`^\[[^]]+\]\[[A-Z]+\]\[[^]]*\]\[[^]]*\] `)

// ReadNotifications includes every available historical log, not just today's file.
func ReadNotifications() ([]Notification, error) {
	logMu.Lock()
	defer logMu.Unlock()
	entries := make(map[string]Notification)
	add := func(entry Notification) { entries[entry.ID] = entry }
	if globalLogDir != "" {
		paths, err := filepath.Glob(filepath.Join(globalLogDir, "*.log"))
		if err != nil {
			return nil, err
		}
		paths = append(paths, filepath.Join(globalLogDir, "notifications.jsonl"))
		for _, path := range paths {
			file, err := os.Open(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 4096), 16<<20)
			var pending *Notification
			flush := func() {
				if pending != nil {
					add(notification(pending.CreatedAt, pending.Level, pending.Message))
					pending = nil
				}
			}
			for scanner.Scan() {
				line := scanner.Text()
				if filepath.Ext(path) == ".jsonl" {
					var entry Notification
					if json.Unmarshal([]byte(line), &entry) == nil && entry.ID != "" {
						add(entry)
					}
				} else if match := notificationLogPattern.FindStringSubmatch(line); match != nil {
					flush()
					entry := notification(match[1], strings.ToLower(match[2]), match[3])
					pending = &entry
				} else if logEntryPattern.MatchString(line) {
					flush()
				} else if pending != nil {
					pending.Message += "\n" + line
				}
			}
			flush()
			scanErr := scanner.Err()
			_ = file.Close()
			if scanErr != nil {
				return nil, scanErr
			}
		}
	}
	for _, entry := range notificationMemory {
		add(entry)
	}
	result := make([]Notification, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, result[i].CreatedAt)
		right, _ := time.Parse(time.RFC3339Nano, result[j].CreatedAt)
		if left.Equal(right) {
			return result[i].ID > result[j].ID
		}
		return left.After(right)
	})
	return result, nil
}

func IsolateNotificationsForTest(dir string) func() {
	logMu.Lock()
	oldDir, oldMemory := globalLogDir, notificationMemory
	globalLogDir, notificationMemory = dir, nil
	logMu.Unlock()
	return func() { logMu.Lock(); globalLogDir, notificationMemory = oldDir, oldMemory; logMu.Unlock() }
}

func ClearNotificationMemoryForTest() {
	logMu.Lock()
	notificationMemory = nil
	logMu.Unlock()
}

func RecordNotificationAtForTest(date, level, message string) {
	logMu.Lock()
	defer logMu.Unlock()
	recordNotificationLocked(notification(date, level, message))
}
