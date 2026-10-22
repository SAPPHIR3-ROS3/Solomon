package chatstore

import (
	"encoding/json"
	"time"
)

type ToolCall struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`

	CheckpointSeq       int    `json:"cp_seq,omitempty"`
	CpSeqSet            bool   `json:"cp_set,omitempty"`
	CheckpointBranchKey string `json:"cp_branch,omitempty"`
}

type Message struct {
	FileChangesRecorded bool                `json:"file_changes_recorded,omitempty"`
	FileChanges         []FileChangeSummary `json:"file_changes,omitempty"`
	ContextKind         string              `json:"context_kind,omitempty"`
	Provider            string              `json:"provider,omitempty"`
	FastMode            *bool               `json:"fast_mode,omitempty"`
	ReasoningEffort     string              `json:"reasoning_effort,omitempty"`
	Model               string              `json:"model,omitempty"`
	GUIHidden           bool                `json:"gui_hidden,omitempty"`
	CreatedAt           time.Time           `json:"created_at,omitempty"`
	Role                string              `json:"role"`
	Content             string              `json:"content"`
	APIContent          string              `json:"api_content,omitempty"`
	ToolCallID          string              `json:"tool_call_id,omitempty"`
	ToolCalls           []ToolCall          `json:"tool_calls,omitempty"`
	ReasoningText       string              `json:"reasoning_text,omitempty"`
	AnthropicThinking   []json.RawMessage   `json:"anthropic_thinking,omitempty"`

	CheckpointSeq       int    `json:"cp_seq,omitempty"`
	CpSeqSet            bool   `json:"cp_set,omitempty"`
	CheckpointBranchKey string `json:"cp_branch,omitempty"`
	CommitOID           string `json:"commit_oid,omitempty"`

	UserPromptTokens          int64   `json:"user_prompt_tokens"`
	ReasoningTokens           int64   `json:"reasoning_tokens"`
	ResponseTokens            int64   `json:"response_tokens"`
	TurnTotalTokens           int64   `json:"turn_total_tokens"`
	PromptTokens              int64   `json:"prompt_tokens,omitempty"`
	CacheUsageTracked         bool    `json:"cache_usage_tracked,omitempty"`
	CachedPromptTokens        int64   `json:"cached_prompt_tokens,omitempty"`
	CacheCreationPromptTokens int64   `json:"cache_creation_prompt_tokens,omitempty"`
	OutputTPS                 float64 `json:"output_tps,omitempty"`
	TTFTSecs                  float64 `json:"ttft_secs,omitempty"`
	PromptTPS                 float64 `json:"prompt_tps,omitempty"`
	ToolWallSecs              float64 `json:"tool_wall_secs,omitempty"`
	TurnWallSecs              float64 `json:"turn_wall_secs,omitempty"`

	TurnDisplaySaved              bool    `json:"turn_display_saved,omitempty"`
	TurnContextTokens             int64   `json:"turn_context_tokens,omitempty"`
	TurnContextEst                bool    `json:"turn_context_est,omitempty"`
	TurnUserTokens                int64   `json:"turn_user_tokens,omitempty"`
	TurnCachedPromptTokens        int64   `json:"turn_cached_prompt_tokens,omitempty"`
	TurnCacheCreationPromptTokens int64   `json:"turn_cache_creation_prompt_tokens,omitempty"`
	TurnReasonTokens              int64   `json:"turn_reason_tokens,omitempty"`
	TurnRespTokens                int64   `json:"turn_resp_tokens,omitempty"`
	TurnTotalDisplay              int64   `json:"turn_total_display,omitempty"`
	TurnOutputTPS                 float64 `json:"turn_output_tps,omitempty"`
	TurnTTFTSecs                  float64 `json:"turn_ttft_secs,omitempty"`
	TurnPromptTPS                 float64 `json:"turn_prompt_tps,omitempty"`
	TurnWallDisplay               float64 `json:"turn_wall_display_secs,omitempty"`
}

type BranchSegment struct {
	ForkAtInclusive int       `json:"fork_at"`
	Messages        []Message `json:"messages"`
}

type UncompactedDump struct {
	CompactAt              time.Time       `json:"compact_at"`
	Messages               []Message       `json:"messages"`
	CheckpointLast         int             `json:"checkpoint_last"`
	CheckpointCP0          bool            `json:"cp0,omitempty"`
	CheckpointBranchSuffix string          `json:"cp_branch_suffix,omitempty"`
	ForkChildCount         map[int]int     `json:"fork_child_count,omitempty"`
	Branches               []BranchSegment `json:"branches,omitempty"`
	LastCommitOID          string          `json:"last_commit_oid,omitempty"`
}

type Session struct {
	FileChanges       map[string]FileChangeLedger `json:"file_changes,omitempty"`
	FileChangesTurn   int                         `json:"file_changes_turn,omitempty"`
	ID                string                      `json:"id"`
	Title             string                      `json:"title"`
	CreatedAt         time.Time                   `json:"created_at"`
	LastMessageAt     time.Time                   `json:"last_message_at"`
	LastUserMessageAt time.Time                   `json:"last_user_message_at,omitempty"`
	Messages          []Message                   `json:"messages"`
	Capabilities      CapabilityRegistry          `json:"capabilities,omitempty"`

	CheckpointLast           int               `json:"checkpoint_last"`
	CheckpointCP0            bool              `json:"cp0,omitempty"`
	CheckpointBranchSuffix   string            `json:"cp_branch_suffix,omitempty"`
	ForkChildCount           map[int]int       `json:"fork_child_count,omitempty"`
	Branches                 []BranchSegment   `json:"branches,omitempty"`
	LastCommitOID            string            `json:"last_commit_oid,omitempty"`
	ImageSeq                 int               `json:"image_seq,omitempty"`
	ImageFiles               map[int]string    `json:"image_files,omitempty"`
	TerminalClips            map[string]string `json:"terminal_clips,omitempty"`
	ActivatedInstructionDirs []string          `json:"activated_instruction_dirs,omitempty"`
	UncompactedRaw           []UncompactedDump `json:"uncompactedRaw,omitempty"`

	PlanningActive   bool   `json:"planning_active,omitempty"`
	ActivePlanName   string `json:"active_plan_name,omitempty"`
	PlanImplementing bool   `json:"plan_implementing,omitempty"`
}

// FileChangeLedger stores only this chat's tool deltas, never workspace totals.
type FileChangeLedger struct {
	AddedLines   map[string]int `json:"added_lines,omitempty"`
	RemovedLines map[string]int `json:"removed_lines,omitempty"`
	Binary       bool           `json:"binary,omitempty"`
	BeforeExists bool           `json:"before_exists"`
	AfterExists  bool           `json:"after_exists"`
}

// FileChangeSummary is a saved view of this chat's pending changes at turn end.
type FileChangeSummary struct {
	Path         string   `json:"path"`
	Added        int      `json:"added"`
	Removed      int      `json:"removed"`
	Binary       bool     `json:"binary,omitempty"`
	AddedLines   []string `json:"addedLines,omitempty"`
	RemovedLines []string `json:"removedLines,omitempty"`
}
