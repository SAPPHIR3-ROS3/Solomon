package chatstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"solomon/internal/logging"
	"solomon/internal/paths"
)

func NewPlaceholderChatID(t time.Time) string {
	u := t.UTC()
	s := u.Format(time.RFC3339Nano)
	s = strings.ReplaceAll(s, ":", "-")
	return "newchat-" + s
}

func IsPlaceholderChatID(id string) bool {
	return strings.HasPrefix(id, "newchat-")
}

type ToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type Session struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	CreatedAt          time.Time `json:"created_at"`
	LastMessageAt      time.Time `json:"last_message_at"`
	LastUserMessageAt  time.Time `json:"last_user_message_at,omitempty"`
	LegacyTools        bool      `json:"legacy_tools,omitempty"`
	Messages           []Message `json:"messages"`
}

func ChatIDHex(title string, ts time.Time) string {
	s := title + "\x00" + ts.UTC().Format(time.RFC3339Nano)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func ChatsDir(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "chats"), nil
}

func SubchatsDir(projectHex string) (string, error) {
	d, err := ChatsDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "subchats"), nil
}

func PlansDir(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "plans"), nil
}

func SessionPath(projectHex, chatIDHex string) (string, error) {
	d, err := ChatsDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, chatIDHex+".json"), nil
}

func WriteSession(projectHex string, s *Session) error {
	p, err := SessionPath(projectHex, s.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "chatstore marshal session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "chatstore write session temp failed", logging.LogOptions{Params: map[string]any{"path": tmp, "err": err.Error()}})
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "chatstore finalize session rename failed", logging.LogOptions{Params: map[string]any{"path": p, "err": err.Error()}})
		return err
	}
	return nil
}

func RenameSessionFile(projectHex, oldID, newID string) error {
	oldPath, err := SessionPath(projectHex, oldID)
	if err != nil {
		return err
	}
	newPath, err := SessionPath(projectHex, newID)
	if err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

func RemoveSessionPath(projectHex, chatID string) error {
	p, err := SessionPath(projectHex, chatID)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func ReadSession(projectHex, chatIDHex string) (*Session, error) {
	p, err := SessionPath(projectHex, chatIDHex)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadAllSessions(projectHex string) ([]*Session, error) {
	d, err := ChatsDir(projectHex)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Session
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		fp := filepath.Join(d, name)
		b, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		var s Session
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		out = append(out, &s)
	}
	return out, nil
}

func lastUserMessageSortTime(s *Session) time.Time {
	if !s.LastUserMessageAt.IsZero() {
		return s.LastUserMessageAt
	}
	return s.LastMessageAt
}

func ListRecent(projectHex string, n int) ([]*Session, error) {
	out, err := loadAllSessions(projectHex)
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastMessageAt.After(out[j].LastMessageAt)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func SessionWithLatestUserMessage(projectHex string) (*Session, error) {
	out, err := loadAllSessions(projectHex)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	sort.Slice(out, func(i, j int) bool {
		return lastUserMessageSortTime(out[i]).After(lastUserMessageSortTime(out[j]))
	})
	return out[0], nil
}

func FindByTitle(projectHex, title string) (*Session, error) {
	all, err := ListRecent(projectHex, 10000)
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if s.Title == title {
			return s, nil
		}
	}
	return nil, os.ErrNotExist
}
