package sdk

type ReadResult struct {
	Path       string
	Content    string
	TotalLines int
	StartLine  int
	EndLine    int
}

type ShellOutput struct {
	Output string
	Exit   int
	Intent string
}

type FetchWebResult struct {
	URL         string
	Status      int
	ContentType string
	Markdown    string
	Title       string
	Metadata    *FetchWebMetadata
}

type FetchWebMetadata struct {
	Provider     string            `json:"provider,omitempty"`
	Adapter      string            `json:"adapter,omitempty"`
	Fallback     bool              `json:"fallback,omitempty"`
	Partial      bool              `json:"partial,omitempty"`
	Attempts     []FetchWebAttempt `json:"attempts,omitempty"`
	ProviderData map[string]any    `json:"providerData,omitempty"`
}

type FetchWebAttempt struct {
	Backend    string `json:"backend"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type GrepLine struct {
	Path string
	Line int
	Text string
}

type GrepCountEntry struct {
	Path  string
	Count int
}

type FindResult struct {
	Files      bool
	Pattern    string
	Path       string
	Matches    []string
	Count      int
	OutputMode string
	Output     string
	Exit       int
}

type WebHit struct {
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Snippet     string         `json:"snippet,omitempty"`
	Content     string         `json:"content,omitempty"`
	Author      string         `json:"author,omitempty"`
	PublishedAt string         `json:"publishedAt,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type WebSearchResult struct {
	Engine       string             `json:"engine"`
	Hits         []WebHit           `json:"hits"`
	HasMore      bool               `json:"hasMore,omitempty"`
	SearxBaseURL string             `json:"searxBaseURL,omitempty"`
	Metadata     *WebSearchMetadata `json:"metadata,omitempty"`
}

type WebSearchMetadata struct {
	Provider          string             `json:"provider,omitempty"`
	Adapter           string             `json:"adapter,omitempty"`
	SessionID         string             `json:"sessionId,omitempty"`
	ProviderRequestID string             `json:"providerRequestId,omitempty"`
	Warnings          []string           `json:"warnings,omitempty"`
	Fallback          bool               `json:"fallback,omitempty"`
	Partial           bool               `json:"partial,omitempty"`
	Attempts          []WebSearchAttempt `json:"attempts,omitempty"`
	ProviderData      map[string]any     `json:"providerData,omitempty"`
}

type WebSearchAttempt struct {
	Backend    string `json:"backend"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type DocsSnippet struct {
	Path      string  `json:"path"`
	Heading   string  `json:"heading"`
	StartLine int     `json:"startLine"`
	EndLine   int     `json:"endLine"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
}

type DocsResult struct {
	Mode    string        `json:"mode"`
	Query   string        `json:"query"`
	Path    string        `json:"path,omitempty"`
	Lines   int           `json:"lines,omitempty"`
	Content string        `json:"content,omitempty"`
	Results []DocsSnippet `json:"results,omitempty"`
}

type ListDirEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

type ListDirResult struct {
	Path    string         `json:"path"`
	Entries []ListDirEntry `json:"entries"`
	Count   int            `json:"count"`
}

type TreeResult struct {
	Path      string `json:"path"`
	Tree      string `json:"tree"`
	Entries   int    `json:"entries"`
	Truncated bool   `json:"truncated,omitempty"`
}

type EditResult struct {
	OK     bool
	Action string
	Reason string
	From   string
	To     string
	Intent string
}
