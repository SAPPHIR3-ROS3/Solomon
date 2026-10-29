package server

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"time"
)

type apiProject struct {
	Chats      []apiChatSummary `json:"chats"`
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	ChatCount  int              `json:"chatCount"`
	TokenStats apiTokenStats    `json:"tokenStats"`
}

type apiChatSummary struct {
	ID            string    `json:"id"`
	LastMessageAt time.Time `json:"lastMessageAt"`
	Title         string    `json:"title"`
}

type apiTokenStats struct {
	User      int64 `json:"user"`
	Reasoning int64 `json:"reasoning"`
	Response  int64 `json:"response"`
	Total     int64 `json:"total"`
}

type apiAtMentionSuggestion struct {
	IsDirectory bool   `json:"isDirectory"`
	Path        string `json:"path"`
	Tag         string `json:"tag"`
}

type apiProjectSidebar struct {
	FastMode        bool         `json:"fastMode"`
	Projects        []apiProject `json:"projects"`
	ReasoningEffort string       `json:"reasoningEffort"`
	UserName        string       `json:"userName"`
}

type fastModeRequest struct {
	FastMode *bool `json:"fastMode"`
}

type apiChat struct {
	CreatedAt    string       `json:"createdAt,omitempty"`
	ID           string       `json:"id"`
	Messages     []apiMessage `json:"messages"`
	Mode         string       `json:"mode,omitempty"`
	ProjectID    string       `json:"projectID,omitempty"`
	Status       string       `json:"status,omitempty"`
	RunStartedAt string       `json:"runStartedAt,omitempty"`
	Title        string       `json:"title"`
}

type apiMessage struct {
	FileChangesRecorded bool                          `json:"fileChangesRecorded,omitempty"`
	FileChanges         []chatstore.FileChangeSummary `json:"fileChanges,omitempty"`
	CreatedAt           string                        `json:"createdAt,omitempty"`
	CheckpointBranch    string                        `json:"checkpointBranch,omitempty"`
	CheckpointSeq       *int                          `json:"checkpointSeq,omitempty"`
	Content             string                        `json:"content"`
	ID                  string                        `json:"id"`
	Images              []apiImage                    `json:"images,omitempty"`
	Kind                string                        `json:"kind,omitempty"`
	Reasoning           string                        `json:"reasoning,omitempty"`
	RetainedMessages    []apiRetainedMessage          `json:"retainedMessages,omitempty"`
	Role                string                        `json:"role"`
	Stats               *apiStats                     `json:"stats,omitempty"`
	Status              string                        `json:"status,omitempty"`
	Summary             string                        `json:"summary,omitempty"`
	ThoughtFor          float64                       `json:"thoughtFor,omitempty"`
	ToolCalls           []apiToolCall                 `json:"toolCalls,omitempty"`
	WorkedFor           float64                       `json:"workedFor,omitempty"`
}

type apiRetainedMessage struct {
	Content string     `json:"content"`
	Images  []apiImage `json:"images,omitempty"`
	Role    string     `json:"role"`
}

type apiImage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type apiStats struct {
	TurnAggregate             bool    `json:"turnAggregate,omitempty"`
	ReasoningEffort           string  `json:"reasoningEffort,omitempty"`
	Provider                  string  `json:"provider,omitempty"`
	Model                     string  `json:"model,omitempty"`
	ContextTokens             int64   `json:"contextTokens"`
	CachedPromptTokens        int64   `json:"cachedPromptTokens"`
	CacheCreationPromptTokens int64   `json:"cacheCreationPromptTokens"`
	OutputTokensPerSecond     float64 `json:"outputTokensPerSecond"`
	PromptTokensPerSecond     float64 `json:"promptTokensPerSecond"`
	ReasoningTokens           int64   `json:"reasoningTokens"`
	ResponseTokens            int64   `json:"responseTokens"`
	TotalTokens               int64   `json:"totalTokens"`
	TTFTSeconds               float64 `json:"ttftSeconds"`
	UserTokens                int64   `json:"userTokens"`
}

type apiToolCall struct {
	CheckpointBranch string             `json:"checkpointBranch,omitempty"`
	CheckpointSeq    *int               `json:"checkpointSeq,omitempty"`
	DefaultOpen      bool               `json:"defaultOpen,omitempty"`
	Delete           bool               `json:"delete,omitempty"`
	Full             bool               `json:"full,omitempty"`
	ID               string             `json:"id"`
	Input            string             `json:"input,omitempty"`
	Intent           string             `json:"intent"`
	Mode             string             `json:"mode,omitempty"`
	Name             string             `json:"name"`
	NewString        string             `json:"newString,omitempty"`
	OldString        string             `json:"oldString,omitempty"`
	Parameters       []apiToolParameter `json:"parameters,omitempty"`
	RenameTo         string             `json:"renameTo,omitempty"`
	Result           map[string]any     `json:"result,omitempty"`
	Status           string             `json:"status,omitempty"`
	Model            string             `json:"model,omitempty"`
	ReasoningEffort  string             `json:"reasoningEffort,omitempty"`
	Provider         string             `json:"provider,omitempty"`
	FastMode         *bool              `json:"fastMode,omitempty"`
	Sync             bool               `json:"sync,omitempty"`
}

type apiToolParameter struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type createProjectRequest struct {
	Path string `json:"path"`
}

type createChatRequest struct {
	Title string `json:"title,omitempty"`
}

type renameChatRequest struct {
	Title string `json:"title"`
}

type sendChatRequest struct {
	Content string          `json:"content"`
	Images  []incomingImage `json:"images,omitempty"`
	Clips   []incomingClip  `json:"clips,omitempty"`
	Surface string          `json:"surface,omitempty"`
}

type incomingClip struct {
	End   int    `json:"end,omitempty"`
	Start int    `json:"start,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Text  string `json:"text,omitempty"`
}

type incomingImage struct {
	Data string `json:"data,omitempty"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type controlSubchatRequest struct {
	Action string `json:"action"`
}
