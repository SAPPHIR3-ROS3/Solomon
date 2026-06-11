package chatstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
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

	CheckpointSeq       int    `json:"cp_seq,omitempty"`
	CpSeqSet            bool   `json:"cp_set,omitempty"`
	CheckpointBranchKey string `json:"cp_branch,omitempty"`
}

type Message struct {
	Role          string     `json:"role"`
	Content       string     `json:"content"`
	APIContent    string     `json:"api_content,omitempty"`
	ToolCallID    string     `json:"tool_call_id,omitempty"`
	ToolCalls     []ToolCall `json:"tool_calls,omitempty"`
	ReasoningText string     `json:"reasoning_text,omitempty"`

	CheckpointSeq       int    `json:"cp_seq,omitempty"`
	CpSeqSet            bool   `json:"cp_set,omitempty"`
	CheckpointBranchKey string `json:"cp_branch,omitempty"`
	CommitOID           string `json:"commit_oid,omitempty"`

	UserPromptTokens   int64   `json:"user_prompt_tokens"`
	ReasoningTokens    int64   `json:"reasoning_tokens"`
	ResponseTokens     int64   `json:"response_tokens"`
	TurnTotalTokens    int64   `json:"turn_total_tokens"`
	PromptTokens       int64   `json:"prompt_tokens,omitempty"`
	CachedPromptTokens int64   `json:"cached_prompt_tokens,omitempty"`
	OutputTPS          float64 `json:"output_tps,omitempty"`
	TTFTSecs           float64 `json:"ttft_secs,omitempty"`
	PromptTPS          float64 `json:"prompt_tps,omitempty"`
	TurnWallSecs       float64 `json:"turn_wall_secs,omitempty"`

	TurnDisplaySaved  bool    `json:"turn_display_saved,omitempty"`
	TurnContextTokens int64   `json:"turn_context_tokens,omitempty"`
	TurnContextEst    bool    `json:"turn_context_est,omitempty"`
	TurnUserTokens    int64   `json:"turn_user_tokens,omitempty"`
	TurnReasonTokens  int64   `json:"turn_reason_tokens,omitempty"`
	TurnRespTokens    int64   `json:"turn_resp_tokens,omitempty"`
	TurnTotalDisplay  int64   `json:"turn_total_display,omitempty"`
	TurnOutputTPS     float64 `json:"turn_output_tps,omitempty"`
	TurnTTFTSecs      float64 `json:"turn_ttft_secs,omitempty"`
	TurnPromptTPS     float64 `json:"turn_prompt_tps,omitempty"`
	TurnWallDisplay   float64 `json:"turn_wall_display_secs,omitempty"`
}

type MainOrphanSegment struct {
	ForkAtInclusive int       `json:"fork_at"`
	Messages        []Message `json:"messages"`
}

type UncompactedDump struct {
	CompactAt              time.Time           `json:"compact_at"`
	Messages               []Message           `json:"messages"`
	CheckpointLast         int                 `json:"checkpoint_last"`
	CheckpointCP0          bool                `json:"cp0,omitempty"`
	CheckpointBranchSuffix string              `json:"cp_branch_suffix,omitempty"`
	ForkChildCount         map[int]int         `json:"fork_child_count,omitempty"`
	MainOrphans            []MainOrphanSegment `json:"main_orphans,omitempty"`
	LastCommitOID          string              `json:"last_commit_oid,omitempty"`
}

type Session struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	CreatedAt         time.Time `json:"created_at"`
	LastMessageAt     time.Time `json:"last_message_at"`
	LastUserMessageAt time.Time `json:"last_user_message_at,omitempty"`
	Messages          []Message `json:"messages"`

	CheckpointLast           int                 `json:"checkpoint_last"`
	CheckpointCP0            bool                `json:"cp0,omitempty"`
	CheckpointBranchSuffix   string              `json:"cp_branch_suffix,omitempty"`
	ForkChildCount           map[int]int         `json:"fork_child_count,omitempty"`
	MainOrphans              []MainOrphanSegment `json:"main_orphans,omitempty"`
	LastCommitOID            string              `json:"last_commit_oid,omitempty"`
	ImageSeq                 int                 `json:"image_seq,omitempty"`
	ImageFiles               map[int]string      `json:"image_files,omitempty"`
	ActivatedInstructionDirs []string            `json:"activated_instruction_dirs,omitempty"`
	UncompactedRaw           []UncompactedDump   `json:"uncompactedRaw,omitempty"`

	PlanningActive   bool   `json:"planning_active,omitempty"`
	ActivePlanName   string `json:"active_plan_name,omitempty"`
	PlanImplementing bool   `json:"plan_implementing,omitempty"`
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

func TempDir(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "temp"), nil
}

func SessionPath(projectHex, chatIDHex string) (string, error) {
	d, err := ChatsDir(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, chatIDHex+".json"), nil
}

func WriteSession(projectHex string, s *Session) (err error) {
	defer func() {
		if err != nil {
			id := ""
			if s != nil {
				id = s.ID
			}
			logging.Log(logging.ERROR_LOG_LEVEL, "chatstore write session failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "chat_id": id, "err": err.Error()}})
		}
	}()
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
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	if err := touchProjectStatsAfterWrite(projectHex, s); err != nil {
		return err
	}
	return nil
}

func RenameSessionFile(projectHex, oldID, newID string) (err error) {
	defer func() {
		if err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "chatstore rename session file failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "old_id": oldID, "new_id": newID, "err": err.Error()}})
		}
	}()
	oldPath, err := SessionPath(projectHex, oldID)
	if err != nil {
		return err
	}
	newPath, err := SessionPath(projectHex, newID)
	if err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	_ = TouchProjectStatsAfterRename(projectHex, oldID, newID)
	return nil
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
	if err != nil {
		return err
	}
	return TouchProjectStatsAfterRemove(projectHex, chatID)
}

func ReadSession(projectHex, chatIDHex string) (sess *Session, err error) {
	defer func() {
		if err != nil && !os.IsNotExist(err) {
			logging.Log(logging.WARNING_LOG_LEVEL, "chatstore read session failed", logging.LogOptions{Params: map[string]any{"project": projectHex, "chat_id": chatIDHex, "err": err.Error()}})
		}
	}()
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
	_ = FinishSessionLoad(&s)
	sess = &s
	return sess, nil
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
		_ = FinishSessionLoad(&s)
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

func assistantMessageHasStoredUsage(m Message) bool {
	return m.TurnDisplaySaved || m.PromptTokens != 0 || m.UserPromptTokens != 0 || m.ReasoningTokens != 0 || m.ResponseTokens != 0 || m.TurnTotalTokens != 0 || m.TurnWallSecs != 0
}

var thinkBlockRes = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<redacted_thinking>(.*?)</redacted_thinking>`),
	regexp.MustCompile(`(?is)<thinking>(.*?)</thinking>`),
	regexp.MustCompile(`(?is)<think>(.*?)</think>`),
	regexp.MustCompile(`(?is)<redacted_reasoning>(.*?)</redacted_reasoning>`),
}

func extractBracketReasoning(content string) (reasoning string, visible string) {
	visible = content
	for _, re := range thinkBlockRes {
		all := re.FindAllStringSubmatch(visible, -1)
		for _, sm := range all {
			if len(sm) > 1 {
				reasoning += sm[1]
			}
		}
		visible = re.ReplaceAllString(visible, "")
	}
	return reasoning, strings.TrimSpace(visible)
}

func AssistantDisplayParts(m Message) (reasoning string, visibleContent string) {
	if rt := strings.TrimSpace(m.ReasoningText); rt != "" {
		_, vis := extractBracketReasoning(m.Content)
		return rt, strings.TrimSpace(vis)
	}
	return extractBracketReasoning(m.Content)
}

func priorNonToolUserForAssistant(msgs []Message, asstIdx int) string {
	for j := asstIdx - 1; j >= 0; j-- {
		if msgs[j].Role != "user" {
			continue
		}
		c := msgs[j].Content
		if strings.HasPrefix(strings.TrimSpace(c), "tool_result(") {
			continue
		}
		return c
	}
	return ""
}

func estimateAssistantTurnTokens(msgs []Message, asstIdx int) (userTok, reasonTok, respTok int64) {
	m := msgs[asstIdx]
	u := priorNonToolUserForAssistant(msgs, asstIdx)
	rText, vis := extractBracketReasoning(m.Content)
	if rt := strings.TrimSpace(m.ReasoningText); rt != "" {
		rText += rt
	}
	model := tokcount.DefaultModel
	toolExtra := m.ToolCallID
	for _, tc := range m.ToolCalls {
		toolExtra += tc.ID + tc.Name + tc.Arguments
	}
	return tokcount.TextTokens(u, model), tokcount.TextTokens(rText, model), tokcount.TextTokens(vis+toolExtra, model)
}

func BackfillAssistantUsageFromTextIfEmpty(m *Message, prior []Message) {
	if m == nil || m.Role != "assistant" {
		return
	}
	if assistantMessageHasStoredUsage(*m) {
		return
	}
	idx := len(prior)
	thread := make([]Message, idx+1)
	copy(thread, prior)
	thread[idx] = *m
	eu, er, es := estimateAssistantTurnTokens(thread, idx)
	m.UserPromptTokens = eu
	m.ReasoningTokens = er
	m.ResponseTokens = es
	m.TurnTotalTokens = eu + er + es
}

