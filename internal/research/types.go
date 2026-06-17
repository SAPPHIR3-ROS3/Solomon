package research

import (
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/apitype"
)

const (
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

const (
	PhasePlanning  = "planning"
	PhaseSearching = "searching"
	PhaseReading   = "reading"
	PhaseAnalyzing = "analyzing"
	PhaseWriting   = "writing"
	PhaseError     = "error"
)

type Finding struct {
	URL      string `json:"url"`
	Title    string `json:"title,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Evidence string `json:"evidence,omitempty"`
	Rational string `json:"rational,omitempty"`
}

type JobStats struct {
	DurationSecs   float64 `json:"duration_secs,omitempty"`
	Rounds         int     `json:"rounds,omitempty"`
	Queries        int     `json:"queries,omitempty"`
	URLs           int     `json:"urls,omitempty"`
	Findings       int     `json:"findings,omitempty"`
	URLReadOK      int     `json:"url_read_ok,omitempty"`
	URLFetchFailed int     `json:"url_fetch_failed,omitempty"`
	URLEmptyContent int    `json:"url_empty_content,omitempty"`
	URLLLMFailed   int     `json:"url_llm_failed,omitempty"`
	URLLowQuality  int     `json:"url_low_quality,omitempty"`
	URLParseFailed int     `json:"url_parse_failed,omitempty"`
	SearchFailures int     `json:"search_failures,omitempty"`
	SearchEngine   string  `json:"search_engine,omitempty"`
	Model          string  `json:"model,omitempty"`
	Category       string  `json:"category,omitempty"`
	PromptTokens   int64   `json:"prompt_tokens,omitempty"`
	ResponseTokens int64   `json:"response_tokens,omitempty"`
	TotalTokens    int64   `json:"total_tokens,omitempty"`
	EstimatedTokens int64  `json:"estimated_tokens,omitempty"`
}

type JobRecord struct {
	ID            string     `json:"id"`
	Slug          string     `json:"slug"`
	Title         string     `json:"title"`
	Question      string     `json:"question"`
	Category      string     `json:"category,omitempty"`
	Status        string     `json:"status"`
	Phase         string     `json:"phase,omitempty"`
	Round         int        `json:"round,omitempty"`
	MaxRounds     int        `json:"max_rounds,omitempty"`
	ParentChatID  string     `json:"parent_chat_id,omitempty"`
	ProjectHex    string     `json:"project_hex"`
	HTMLPath      string     `json:"html_path,omitempty"`
	ResearchPlan  string     `json:"research_plan,omitempty"`
	EvolvingReport string    `json:"evolving_report,omitempty"`
	Findings      []Finding  `json:"findings,omitempty"`
	QueriesUsed   []string   `json:"queries_used,omitempty"`
	URLsFetched   []string     `json:"urls_fetched,omitempty"`
	URLAttempts   []URLAttempt `json:"url_attempts,omitempty"`
	Stats         JobStats     `json:"stats,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    time.Time  `json:"finished_at,omitempty"`
}

type ProgressEvent struct {
	Phase         string
	Round         int
	MaxRounds     int
	TotalQueries  int
	TotalSources  int
	TotalFindings int
	QueryPreview  string
	URL           string
	Title         string
	Message       string
}

type LLMCaller interface {
	Complete(userPrompt string, maxTokens int) (string, apitype.UsageStats, error)
}

type ProgressFn func(ProgressEvent)

type CancelFn func() bool

type EngineResumeState struct {
	Plan        string
	Category    string
	Report      string
	Findings    []Finding
	Round       int
	QueriesUsed []string
	URLsFetched []string
}
