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
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type WebSearchResult struct {
	Engine       string   `json:"engine"`
	Hits         []WebHit `json:"hits"`
	HasMore      bool     `json:"hasMore,omitempty"`
	SearxBaseURL string   `json:"searxBaseURL,omitempty"`
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

type EditResult struct {
	OK     bool
	Action string
	Reason string
	From   string
	To     string
	Intent string
}
