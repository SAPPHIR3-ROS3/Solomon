package chatstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
)

const projectStatsVersion = 1

type chatUsageTotals struct {
	UserSum   int64 `json:"user_sum"`
	ReasonSum int64 `json:"reason_sum"`
	RespSum   int64 `json:"resp_sum"`
}

type projectStatsFile struct {
	Version int                        `json:"version"`
	Chats   map[string]chatUsageTotals `json:"chats"`
}

var projectStatsLocks sync.Map

func projectStatsLock(projectHex string) *sync.Mutex {
	v, _ := projectStatsLocks.LoadOrStore(projectHex, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func projectStatsPath(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "welcome_stats.json"), nil
}

func sessionUsageTotals(s *Session) chatUsageTotals {
	var out chatUsageTotals
	if s == nil {
		return out
	}
	for j, m := range s.Messages {
		if m.Role != "assistant" {
			continue
		}
		if assistantMessageHasStoredUsage(m) {
			out.UserSum += m.UserPromptTokens
			out.ReasonSum += m.ReasoningTokens
			out.RespSum += m.ResponseTokens
			continue
		}
		eu, er, es := estimateAssistantTurnTokens(s.Messages, j)
		out.UserSum += eu
		out.ReasonSum += er
		out.RespSum += es
	}
	return out
}

func (f *projectStatsFile) sums() (chatCount int, userSum, reasonSum, respSum int64) {
	if f == nil || len(f.Chats) == 0 {
		return 0, 0, 0, 0
	}
	chatCount = len(f.Chats)
	for _, t := range f.Chats {
		userSum += t.UserSum
		reasonSum += t.ReasonSum
		respSum += t.RespSum
	}
	return chatCount, userSum, reasonSum, respSum
}

func loadProjectStatsFile(projectHex string) (*projectStatsFile, error) {
	p, err := projectStatsPath(projectHex)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f projectStatsFile
	if err := json.Unmarshal(b, &f); err != nil || f.Version != projectStatsVersion || f.Chats == nil {
		return nil, nil
	}
	return &f, nil
}

func saveProjectStatsFile(projectHex string, f *projectStatsFile) error {
	if f == nil {
		return nil
	}
	f.Version = projectStatsVersion
	if f.Chats == nil {
		f.Chats = map[string]chatUsageTotals{}
	}
	p, err := projectStatsPath(projectHex)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func rebuildProjectStatsLocked(projectHex string) error {
	sessions, err := loadAllSessions(projectHex)
	if err != nil {
		return err
	}
	f := &projectStatsFile{Version: projectStatsVersion, Chats: map[string]chatUsageTotals{}}
	for _, s := range sessions {
		if s == nil || s.ID == "" {
			continue
		}
		f.Chats[s.ID] = sessionUsageTotals(s)
	}
	return saveProjectStatsFile(projectHex, f)
}

func RebuildProjectStats(projectHex string) error {
	mu := projectStatsLock(projectHex)
	mu.Lock()
	defer mu.Unlock()
	return rebuildProjectStatsLocked(projectHex)
}

func updateProjectStatsLocked(projectHex string, chatID string, totals chatUsageTotals) error {
	f, err := loadProjectStatsFile(projectHex)
	if err != nil {
		return err
	}
	if f == nil {
		return rebuildProjectStatsLocked(projectHex)
	}
	f.Chats[chatID] = totals
	return saveProjectStatsFile(projectHex, f)
}

func removeProjectStatsChatLocked(projectHex, chatID string) error {
	f, err := loadProjectStatsFile(projectHex)
	if err != nil {
		return err
	}
	if f == nil || f.Chats == nil {
		return nil
	}
	if _, ok := f.Chats[chatID]; !ok {
		return nil
	}
	delete(f.Chats, chatID)
	return saveProjectStatsFile(projectHex, f)
}

func renameProjectStatsChatLocked(projectHex, oldID, newID string) error {
	if oldID == "" || newID == "" || oldID == newID {
		return nil
	}
	f, err := loadProjectStatsFile(projectHex)
	if err != nil {
		return err
	}
	if f == nil || f.Chats == nil {
		return nil
	}
	t, ok := f.Chats[oldID]
	if !ok {
		return nil
	}
	delete(f.Chats, oldID)
	f.Chats[newID] = t
	return saveProjectStatsFile(projectHex, f)
}

func touchProjectStatsAfterWrite(projectHex string, s *Session) error {
	if s == nil || s.ID == "" {
		return nil
	}
	mu := projectStatsLock(projectHex)
	mu.Lock()
	defer mu.Unlock()
	return updateProjectStatsLocked(projectHex, s.ID, sessionUsageTotals(s))
}

func TouchProjectStatsAfterRemove(projectHex, chatID string) error {
	if chatID == "" {
		return nil
	}
	mu := projectStatsLock(projectHex)
	mu.Lock()
	defer mu.Unlock()
	return removeProjectStatsChatLocked(projectHex, chatID)
}

func TouchProjectStatsAfterRename(projectHex, oldID, newID string) error {
	mu := projectStatsLock(projectHex)
	mu.Lock()
	defer mu.Unlock()
	return renameProjectStatsChatLocked(projectHex, oldID, newID)
}

func ProjectWelcomeStats(projectHex string) (chatCount int, userSum, reasonSum, respSum int64, err error) {
	f, err := loadProjectStatsFile(projectHex)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if f != nil {
		n, u, r, resp := f.sums()
		return n, u, r, resp, nil
	}
	if err := RebuildProjectStats(projectHex); err != nil {
		return 0, 0, 0, 0, err
	}
	f, err = loadProjectStatsFile(projectHex)
	if err != nil || f == nil {
		return 0, 0, 0, 0, err
	}
	n, u, r, resp := f.sums()
	return n, u, r, resp, nil
}
