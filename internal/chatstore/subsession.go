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

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

const (
	SubOriginParent    = "parent"
	SubOriginScheduled = "scheduled"
)

const (
	SubStatusRunning   = "running"
	SubStatusPaused    = "paused"
	SubStatusDone      = "done"
	SubStatusCancelled = "cancelled"
	SubStatusQueued    = "queued"
)

type QueuedTask struct {
	Task          string    `json:"task"`
	SysPromptPath string    `json:"sys_prompt_path,omitempty"`
	EnqueuedAt    time.Time `json:"enqueued_at"`
}

type PendingSubagentSpawn struct {
	RequesterSubchatID string          `json:"requester_subchat_id"`
	RequesterOrigin    string          `json:"requester_origin"`
	ParentChatID       string          `json:"parent_chat_id,omitempty"`
	ProjectHex         string          `json:"project_hex,omitempty"`
	SysPromptPath      string          `json:"sys_prompt_path"`
	Task               string          `json:"task"`
	Resume             string          `json:"resume,omitempty"`
	RunInBackground    bool            `json:"run_in_background,omitempty"`
	ReasoningEffort    string          `json:"reasoning_effort,omitempty"`
	RoleProvider       string          `json:"role_provider,omitempty"`
	RoleModel          string          `json:"role_model,omitempty"`
	ToolCall           ToolCall        `json:"tool_call"`
	SpawnISO           string          `json:"spawn_iso"`
	NotifyNewChat      bool            `json:"notify_new_chat,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type SubSession struct {
	ID                string            `json:"id"`
	Title             string            `json:"title"`
	CreatedAt         time.Time         `json:"created_at"`
	LastMessageAt     time.Time         `json:"last_message_at"`
	Messages          []Message         `json:"messages"`
	ImageSeq          int               `json:"image_seq,omitempty"`
	ImageFiles        map[int]string    `json:"image_files,omitempty"`
	ParentChatID      string            `json:"parent_chat_id,omitempty"`
	ParentToolCallID  string            `json:"parent_tool_call_id,omitempty"`
	ProjectHex        string            `json:"project_hex,omitempty"`
	SysPromptPath     string            `json:"sys_prompt_path,omitempty"`
	Origin            string            `json:"origin"`
	ScheduledAt       *time.Time        `json:"scheduled_at,omitempty"`
	PersistContext    bool              `json:"persist_context,omitempty"`
	Status            string            `json:"status"`
	TaskQueue         []QueuedTask      `json:"task_queue,omitempty"`
	ReasoningEffort   string            `json:"reasoning_effort,omitempty"`
	RoleProvider      string            `json:"role_provider,omitempty"`
	RoleModel         string            `json:"role_model,omitempty"`
	PendingSpawns     []PendingSubagentSpawn `json:"pending_spawns,omitempty"`
}

type ActiveSubagentEntry struct {
	ID         string    `json:"id"`
	Origin     string    `json:"origin"`
	Status     string    `json:"status"`
	SessionPath string   `json:"session_path"`
	ProjectHex string    `json:"project_hex,omitempty"`
	SpawnedAt  time.Time `json:"spawned_at"`
}

type ActiveSubagentsFile struct {
	Agents []ActiveSubagentEntry `json:"agents"`
}

func SubchatID(parentChatID string, tc ToolCall, spawn time.Time) string {
	payload := struct {
		ParentChatID string   `json:"parent_chat_id"`
		ToolCall     ToolCall `json:"tool_call"`
		SpawnISO     string   `json:"spawn_iso"`
	}{
		ParentChatID: parentChatID,
		ToolCall:     tc,
		SpawnISO:     spawn.UTC().Format(time.RFC3339Nano),
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func SubSessionPath(projectHex, origin, id string) (string, error) {
	if origin == SubOriginScheduled {
		return paths.ScheduledSubagentPath(id)
	}
	return SubchatPath(projectHex, id)
}

func writeJSONAtomic(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func WriteSubSession(projectHex string, s *SubSession) (err error) {
	defer func() {
		if err != nil {
			id := ""
			if s != nil {
				id = s.ID
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "chatstore write subsession failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "subchat_id": id, "err": err.Error()}})
		}
	}()
	if s == nil || s.ID == "" {
		return nil
	}
	p, err := SubSessionPath(projectHex, s.Origin, s.ID)
	if err != nil {
		return err
	}
	return writeJSONAtomic(p, s)
}

func ReadSubSession(projectHex, origin, id string) (*SubSession, error) {
	p, err := SubSessionPath(projectHex, origin, id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s SubSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func FindSubSessionByID(projectHex, id string) (*SubSession, error) {
	if s, err := ReadSubSession(projectHex, SubOriginParent, id); err == nil {
		return s, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return ReadSubSession(projectHex, SubOriginScheduled, id)
}

func loadSubSessionsFromDir(dir string, origin string, projectHex string) ([]*SubSession, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*SubSession
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if e.Name() == "activeSubagents.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s SubSession
		if err := json.Unmarshal(b, &s); err != nil {
			continue
		}
		if s.Origin == "" {
			s.Origin = origin
		}
		if s.ProjectHex == "" && projectHex != "" {
			s.ProjectHex = projectHex
		}
		out = append(out, &s)
	}
	return out, nil
}

func ListSubSessions(projectHex string) ([]*SubSession, error) {
	var all []*SubSession
	if projectHex != "" {
		d, err := SubchatsDir(projectHex)
		if err != nil {
			return nil, err
		}
		parent, err := loadSubSessionsFromDir(d, SubOriginParent, projectHex)
		if err != nil {
			return nil, err
		}
		all = append(all, parent...)
	}
	sd, err := paths.SubagentsDir()
	if err != nil {
		return nil, err
	}
	sched, err := loadSubSessionsFromDir(sd, SubOriginScheduled, "")
	if err != nil {
		return nil, err
	}
	all = append(all, sched...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all, nil
}

func ReadActiveSubagents() (*ActiveSubagentsFile, error) {
	p, err := paths.ActiveSubagentsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &ActiveSubagentsFile{}, nil
		}
		return nil, err
	}
	var f ActiveSubagentsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func WriteActiveSubagents(f *ActiveSubagentsFile) error {
	if err := paths.EnsureSubagentsDir(); err != nil {
		return err
	}
	p, err := paths.ActiveSubagentsPath()
	if err != nil {
		return err
	}
	if f == nil {
		f = &ActiveSubagentsFile{}
	}
	return writeJSONAtomic(p, f)
}

func ApplyTurnUsageDisplayToLastSubAssistant(s *SubSession, ctxTok, usrTok int64, ctxEst bool, reasonTok, respTok, totalTok int64, outputTPS, ttftSecs, promptTPS, turnWallSecs float64) {
	if s == nil {
		return
	}
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role != "assistant" {
			continue
		}
		m := &s.Messages[i]
		m.TurnDisplaySaved = true
		m.TurnContextTokens = ctxTok
		m.TurnContextEst = ctxEst
		m.TurnUserTokens = usrTok
		m.TurnReasonTokens = reasonTok
		m.TurnRespTokens = respTok
		m.TurnTotalDisplay = totalTok
		m.TurnOutputTPS = outputTPS
		m.TurnTTFTSecs = ttftSecs
		m.TurnPromptTPS = promptTPS
		m.TurnWallDisplay = turnWallSecs
		return
	}
}

func SubSessionRunning(status string) bool {
	return status == SubStatusRunning || status == SubStatusQueued
}
