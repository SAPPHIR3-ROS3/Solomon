package agentruntime

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint/staging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/search"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/webfetch"
	readline "github.com/chzyer/readline"
	"github.com/openai/openai-go/v2"
	"io"
	"sync"
)

type Runtime struct {
	commandOnly bool

	RL *readline.Instance

	Client  openai.Client
	Backend llm.CompletionBackend
	Model   string
	Cfg     *config.Root
	Prov    *config.Provider

	ProjHex  string
	ProjRoot string

	Mode string

	Session *chatstore.Session

	CompactionThresholdTokens int64

	EphemeralSession bool

	chatPersistMu              sync.Mutex
	deferredTitleScheduleMu    sync.Mutex
	deferredTitleWorkerRunning bool
	sessionFileCreated         bool
	replInputPrefillMu         sync.Mutex
	replInputPrefill           string

	Out io.Writer

	MCP         *solomonmcp.Manager
	Cloak       *cloak.Client
	CloakShared bool
	WebSearch   search.Engine
	WebFetch    webfetch.Fetcher

	sandboxRetained bool

	ReplShellFirst bool

	EventSink       cievents.Sink
	FailOnToolError bool

	Surface string

	ciPrompt       string
	ciTurn         int
	ciToolErr      bool
	ciFinalContent string

	ToolOut *tooloutput.Service

	checkpointMu        sync.Mutex
	currentToolCpSeq    int
	stagingCache        *staging.Store
	stagingCacheSession string

	Instructions *instructions.Loader

	providerReady chan struct{}

	updateMu       sync.Mutex
	updateChecked  bool
	updateNotice   *updater.Notice
	updateCheckErr error

	pendingUpdateTag string

	atSkipMu     sync.Mutex
	atSkipNotify *atmention.Notifier
	atSkipShown  bool

	nestedMu                  sync.Mutex
	nestedState               *activeNestedState
	currentSubagentToolCallID string
	roleBackends              roleBackendCache

	replSessFileLock   *chatstore.SessionFileLock
	replSessFileLockID string
}
