package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

const DefaultSubagentTimeoutMinutes = 20

const DefaultResearchMaxRounds = 8

const DefaultResearchMaxURLsPerRound = 3

const DefaultResearchMaxContentChars = 15000

const DefaultResearchMinRounds = 2

const DefaultResponseLanguage = "English"

const DefaultCompactionThresholdTokens int64 = 131072

const MinCompactionThresholdTokens int64 = 32768

const DefaultSkillSearchMinNormalizedScore = 0.05

const DefaultDocSearchMinNormalizedScore = 0.05

const DefaultDocSearchFullArticleScore = 0.9

const DefaultWebSearchEngine = "duckduckgo"

const DefaultServerPort = 64000

const ServerPortEnv = "SOLOMON_SERVER_PORT"

const DefaultToolOutputMaxBytes = 65536

const DefaultToolOutputMaxLines = 2048

const APIProtocolOpenAI = "openai"

const APIProtocolAnthropic = "anthropic"

type Provider struct {
	Name              string `toml:"-"`
	BaseURL           string `toml:"base_url"`
	APIKey            string `toml:"api_key"`
	APIProtocol       string `toml:"api_protocol,omitempty"`
	AuthKind          string `toml:"auth_kind,omitempty"`
	OAuthAccessToken  string `toml:"oauth_access_token,omitempty"`
	OAuthRefreshToken string `toml:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    string `toml:"oauth_expires_at,omitempty"`
	OAuthAccountID    string `toml:"oauth_account_id,omitempty"`
}

type Current struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

type RecentModelUse struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

// ModelMode is the reasoning effort and fast mode last used with one provider model.
type ModelMode struct {
	ReasoningEffort string `toml:"reasoning_effort,omitempty"`
	FastMode        *bool  `toml:"fast_mode,omitempty"`
}

type ToolOutput struct {
	MaxBytes int `toml:"max_bytes,omitempty"`
	MaxLines int `toml:"max_lines,omitempty"`
}

type Tools struct {
	Legacy              bool `toml:"legacy,omitempty"`
	LegacyForce         bool `toml:"legacy_force,omitempty"`
	CursorInternalTools bool `toml:"cursor_internal_tools,omitempty"`
}

type Root struct {
	GUI                       GUISettings `toml:"gui"`
	guiSnapshot               string
	UserName                  string                          `toml:"user_name"`
	Providers                 map[string]*Provider            `toml:"-"`
	Current                   Current                         `toml:"current"`
	RecentModels              map[string][]string             `toml:"recent_models,omitempty"`
	HiddenModels              map[string][]string             `toml:"hidden_models,omitempty"`
	SubagentTimeoutMinutes    int                             `toml:"subagent_timeout_minutes"`
	ReasoningEffort           string                          `toml:"reasoning_effort"`
	SubagentReasoningEffort   string                          `toml:"subagent_reasoning_effort"`
	FastMode                  *bool                           `toml:"fast_mode,omitempty"`
	ModelModes                map[string]map[string]ModelMode `toml:"model_modes,omitempty"`
	LogLevel                  string                          `toml:"log_level"`
	MaxResponseTokens         int                             `toml:"max_response_tokens"`
	ShowFileChanges           *bool                           `toml:"show_file_changes"`
	ShowThinking              bool                            `toml:"show_thinking"`
	Anonymize                 bool                            `toml:"anonymize,omitempty"`
	Tools                     Tools                           `toml:"tools,omitempty"`
	ShowUsageStats            *bool                           `toml:"show_usage_stats"`
	ResponseLanguage          string                          `toml:"response_language"`
	CompactionThresholdTokens int64                           `toml:"compaction_threshold_tokens"`
	SkillSearchMinNorm        *float64                        `toml:"skill_search_min_normalized_score,omitempty"`
	DocSearchMinNorm          *float64                        `toml:"doc_search_min_normalized_score,omitempty"`
	DocSearchFullArticleScore *float64                        `toml:"doc_search_full_article_score,omitempty"`
	WebSearchEngine           string                          `toml:"web_search_engine,omitempty"`
	Server                    ServerSettings                  `toml:"server,omitempty"`
	ServerPort                int                             `toml:"server_port,omitempty"`
	WebSearchAPIKey           string                          `toml:"web_search_api_key,omitempty"`
	WebSearchBaseURL          string                          `toml:"web_search_base_url,omitempty"`
	WebSearchCX               string                          `toml:"web_search_cx,omitempty"`
	ResearchMaxRounds         int                             `toml:"research_max_rounds,omitempty"`
	ResearchMaxURLsPerRound   int                             `toml:"research_max_urls_per_round,omitempty"`
	ResearchMaxContentChars   int                             `toml:"research_max_content_chars,omitempty"`
	ToolOutput                ToolOutput                      `toml:"tool_output,omitempty"`
	APIResilience             APIResilienceConfig             `toml:"api_resilience,omitempty"`
	WebFetch                  WebFetchConfig                  `toml:"web_fetch,omitempty"`
	Export                    Export                          `toml:"export,omitempty"`
	Roles                     Roles                           `toml:"roles,omitempty"`
	AutoUpdate                *bool                           `toml:"autoupdate,omitempty"`
	InstallLayout             string                          `toml:"install_layout,omitempty"`
	PromptTemplates           map[string]string               `toml:"prompt_templates,omitempty"`
	PromptTemplateModTime     map[string]int64                `toml:"prompt_template_mtime,omitempty"`
}

func (r *Root) AutoUpdateEnabled() bool {
	return r != nil && r.AutoUpdate != nil && *r.AutoUpdate
}

func (r *Root) EffectiveInstallLayout() string {
	if r != nil && r.InstallLayout == "standalone" {
		return "standalone"
	}
	return "split"
}

func (r *Root) LegacyToolsEnabled() bool {
	return r != nil && r.Tools.Legacy
}

func (r *Root) LegacyToolsForceEnabled() bool {
	return r.LegacyToolsEnabled() && r.Tools.LegacyForce
}

func (r *Root) CursorInternalToolsEnabled() bool {
	return false
}

func (r *Root) EffectiveWebSearchEngine() string {
	if r == nil {
		return DefaultWebSearchEngine
	}
	v := strings.TrimSpace(r.WebSearchEngine)
	if v == "" {
		return DefaultWebSearchEngine
	}
	return v
}

func (r *Root) EffectiveResponseLanguage() string {
	if r == nil {
		return DefaultResponseLanguage
	}
	s := strings.TrimSpace(r.ResponseLanguage)
	if s == "" {
		return DefaultResponseLanguage
	}
	return s
}

func (r *Root) UsageStatsEnabled() bool {
	if r == nil || r.ShowUsageStats == nil {
		return true
	}
	return *r.ShowUsageStats
}

func SubagentTimeout(r *Root) int {
	n := r.SubagentTimeoutMinutes
	if n <= 0 {
		return DefaultSubagentTimeoutMinutes
	}
	return n
}

func EffectiveResearchMaxRounds(r *Root) int {
	if r == nil || r.ResearchMaxRounds <= 0 {
		return DefaultResearchMaxRounds
	}
	return r.ResearchMaxRounds
}

func EffectiveResearchMaxURLsPerRound(r *Root) int {
	if r == nil || r.ResearchMaxURLsPerRound <= 0 {
		return DefaultResearchMaxURLsPerRound
	}
	return r.ResearchMaxURLsPerRound
}

func EffectiveResearchMaxContentChars(r *Root) int {
	if r == nil || r.ResearchMaxContentChars <= 0 {
		return DefaultResearchMaxContentChars
	}
	return r.ResearchMaxContentChars
}

func ResearchMaxTimeSeconds(r *Root) int {
	return SubagentTimeout(r) * 60
}

func EffectiveCompactionThresholdTokens(r *Root) int64 {
	if r == nil {
		return DefaultCompactionThresholdTokens
	}
	n := r.CompactionThresholdTokens
	if n <= 0 || n < MinCompactionThresholdTokens {
		return DefaultCompactionThresholdTokens
	}
	return n
}

func EffectiveSkillSearchMinNorm(r *Root) float64 {
	if r == nil || r.SkillSearchMinNorm == nil {
		return DefaultSkillSearchMinNormalizedScore
	}
	v := *r.SkillSearchMinNorm
	if v < 0 || v > 1 {
		return DefaultSkillSearchMinNormalizedScore
	}
	return v
}

func EffectiveDocSearchMinNorm(r *Root) float64 {
	if r == nil || r.DocSearchMinNorm == nil {
		return DefaultDocSearchMinNormalizedScore
	}
	v := *r.DocSearchMinNorm
	if v < 0 || v > 1 {
		return DefaultDocSearchMinNormalizedScore
	}
	return v
}

func EffectiveDocSearchFullArticleScore(r *Root) float64 {
	if r == nil || r.DocSearchFullArticleScore == nil {
		return DefaultDocSearchFullArticleScore
	}
	v := *r.DocSearchFullArticleScore
	if v < 0 || v > 1 {
		return DefaultDocSearchFullArticleScore
	}
	return v
}

func ClampTimeoutMinutes(n int) error {
	if n < 1 || n > 180 {
		return fmt.Errorf("timeout must be between 1 and 180 minutes")
	}
	return nil
}

func (r *Root) EffectiveFastMode() bool {
	return r == nil || r.FastMode == nil || *r.FastMode
}

func FastModeSupportedByProvider(p *Provider) bool {
	// ChatGPT Sub uses OpenAI's service tier, Claude Sub uses Anthropic's
	// speed beta, and Cursor uses Solomon's provider-specific flag.
	return p != nil && (p.IsChatGPTSub() || p.IsClaudeSub() || p.IsCursorSub() || p.IsCursorAPI())
}

func (r *Root) FastModeEnabledForProvider(p *Provider) bool {
	return FastModeSupportedByProvider(p) && r.EffectiveFastMode()
}

func (r *Root) ModelDisplayName(p *Provider, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if p != nil && (p.IsCursorSub() || p.IsCursorAPI()) {
		low := strings.ToLower(model)
		if strings.HasPrefix(low, "grok") && !strings.HasPrefix(low, "cursor-") {
			model = "cursor-" + model
		}
	}
	out := fmt.Sprintf("%s (%s)", model, r.ReasoningEffortDisplayLabel())
	if r.FastModeEnabledForProvider(p) {
		out += " (fast)"
	}
	return out
}

func (p *Provider) EffectiveAPIProtocol() string {
	if p == nil {
		return APIProtocolOpenAI
	}
	switch strings.TrimSpace(p.APIProtocol) {
	case APIProtocolAnthropic:
		return APIProtocolAnthropic
	default:
		return APIProtocolOpenAI
	}
}

func (p *Provider) IsAnthropic() bool {
	return p != nil && p.EffectiveAPIProtocol() == APIProtocolAnthropic
}

// providerBaseWithScheme accepts bare IP addresses and host:port endpoints for
// local or self-hosted providers. Explicit URL schemes are validated by callers.
func providerBaseWithScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse("http://" + raw)
	if err != nil || u.User != nil {
		return raw
	}
	if host, port, err := net.SplitHostPort(u.Host); err == nil && host != "" {
		if n, err := strconv.Atoi(port); err == nil && n > 0 && n <= 65535 {
			return "http://" + raw
		}
	}
	if net.ParseIP(u.Hostname()) != nil || u.Host == "localhost" {
		if u.Port() == "" {
			return "http://" + raw
		}
	}
	return raw
}

func NormalizeAnthropicBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty base url")
	}
	u, err := url.Parse(providerBaseWithScheme(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	p := strings.TrimSuffix(u.Path, "/")
	if p == "" || p == "/" {
		u.Path = "/"
	} else {
		u.Path = p + "/"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func NormalizeAPIBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty base url")
	}
	u, err := url.Parse(providerBaseWithScheme(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	p := strings.TrimSuffix(u.Path, "/")
	switch {
	case p == "" || p == "/":
		p = "/v1"
	case strings.HasSuffix(p, "/v1"):
		break
	default:
		p = p + "/v1"
	}
	u.Path = p + "/"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

const recentModelUseCap = 64

func RunWizardIfNeeded(stdin io.Reader) (*Root, error) {
	_ = stdin
	r, err := LoadOptional()
	if err != nil {
		return nil, err
	}
	if NeedsOnboard(r) {
		return nil, fmt.Errorf("config not set up; run solomon and use /onboard")
	}
	return r, nil
}

func ResolveProvider(r *Root) (*Provider, error) {
	if r == nil {
		return nil, errors.New("nil config")
	}
	name := r.Current.Provider
	if p := ProviderByName(r, name); p != nil {
		return p, nil
	}
	if first := FirstProviderName(r); first != "" {
		r.Current.Provider = first
		p := ProviderByName(r, first)
		if err := Save(r); err != nil {
			logging.Log(logging.WARNING_LOG_LEVEL, "save config after auto-select provider failed", logging.LogOptions{Params: map[string]any{"provider": first, "err": err.Error()}})
			return p, err
		}
		logging.Log(logging.INFO_LOG_LEVEL, "auto-selected provider", logging.LogOptions{Params: map[string]any{"provider": first}})
		return p, nil
	}
	logging.Log(logging.ERROR_LOG_LEVEL, "no providers in config", logging.LogOptions{Params: nil})
	return nil, errors.New("no providers in config")
}

// FileChangesEnabled defaults to true while preserving an explicit opt-out.
func (r *Root) FileChangesEnabled() bool {
	return r == nil || r.ShowFileChanges == nil || *r.ShowFileChanges
}
