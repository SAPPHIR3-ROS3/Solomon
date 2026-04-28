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

	"solomon/internal/paths"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Session struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	CreatedAt      time.Time `json:"created_at"`
	LastMessageAt  time.Time `json:"last_message_at"`
	Messages       []Message `json:"messages"`
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
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
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

func ListRecent(projectHex string, n int) ([]*Session, error) {
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastMessageAt.After(out[j].LastMessageAt)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out, nil
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
