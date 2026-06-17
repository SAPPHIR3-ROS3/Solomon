package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/openai/openai-go/v2/shared"
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
	UserName                  string               `toml:"user_name"`
	Providers                 map[string]*Provider `toml:"-"`
	Current                   Current              `toml:"current"`
	RecentModels              map[string][]string  `toml:"recent_models,omitempty"`
	SubagentTimeoutMinutes    int                  `toml:"subagent_timeout_minutes"`
	ReasoningEffort           string               `toml:"reasoning_effort"`
	SubagentReasoningEffort   string               `toml:"subagent_reasoning_effort"`
	FastMode                  *bool                `toml:"fast_mode,omitempty"`
	LogLevel                  string               `toml:"log_level"`
	MaxResponseTokens         int                  `toml:"max_response_tokens"`
	ShowThinking              bool                 `toml:"show_thinking"`
	Tools                     Tools                `toml:"tools,omitempty"`
	ShowUsageStats            *bool                `toml:"show_usage_stats"`
	ResponseLanguage          string               `toml:"response_language"`
	CompactionThresholdTokens int64                `toml:"compaction_threshold_tokens"`
	SkillSearchMinNorm        *float64             `toml:"skill_search_min_normalized_score,omitempty"`
	DocSearchMinNorm          *float64             `toml:"doc_search_min_normalized_score,omitempty"`
	DocSearchFullArticleScore *float64             `toml:"doc_search_full_article_score,omitempty"`
	WebSearchEngine           string               `toml:"web_search_engine,omitempty"`
	WebSearchAPIKey           string               `toml:"web_search_api_key,omitempty"`
	WebSearchBaseURL          string               `toml:"web_search_base_url,omitempty"`
	WebSearchCX               string               `toml:"web_search_cx,omitempty"`
	ResearchMaxRounds         int                  `toml:"research_max_rounds,omitempty"`
	ResearchMaxURLsPerRound   int                  `toml:"research_max_urls_per_round,omitempty"`
	ResearchMaxContentChars   int                  `toml:"research_max_content_chars,omitempty"`
	ToolOutput                ToolOutput           `toml:"tool_output,omitempty"`
	APIResilience             APIResilienceConfig  `toml:"api_resilience,omitempty"`
	WebFetch                  WebFetchConfig       `toml:"web_fetch,omitempty"`
	AutoUpdate                *bool                `toml:"autoupdate,omitempty"`
	PromptTemplates           map[string]string    `toml:"prompt_templates,omitempty"`
}

func (r *Root) AutoUpdateEnabled() bool {
	return r != nil && r.AutoUpdate != nil && *r.AutoUpdate
}

func (r *Root) LegacyToolsEnabled() bool {
	return r != nil && r.Tools.Legacy
}

func (r *Root) LegacyToolsForceEnabled() bool {
	return r.LegacyToolsEnabled() && r.Tools.LegacyForce
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

func ParseReasoningEffortToken(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "none":
		return "none", nil
	case "low":
		return "low", nil
	case "med", "medium":
		return "medium", nil
	case "high":
		return "high", nil
	default:
		return "", fmt.Errorf("reasoning must be none, low, med, or high")
	}
}

func (r *Root) ReasoningEffortIsNone() bool {
	if r == nil {
		return false
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	return err == nil && c == "none"
}

func (r *Root) GlobalReasoningEffort() shared.ReasoningEffort {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	if err != nil {
		return ""
	}
	switch c {
	case "none":
		return shared.ReasoningEffort("none")
	case "low":
		return shared.ReasoningEffortLow
	case "medium":
		return shared.ReasoningEffortMedium
	case "high":
		return shared.ReasoningEffortHigh
	default:
		return ""
	}
}

func (r *Root) ReasoningEffortLabel() string {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.ReasoningEffort)
	if err != nil {
		return ""
	}
	return c
}

func (r *Root) ReasoningEffortDisplayLabel() string {
	if lbl := r.ReasoningEffortLabel(); lbl != "" {
		return lbl
	}
	return "none"
}

func (r *Root) SubagentReasoningEffortIsNone() bool {
	if r == nil {
		return true
	}
	if strings.TrimSpace(r.SubagentReasoningEffort) == "" {
		return true
	}
	c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
	return err == nil && c == "none"
}

func (r *Root) SubagentReasoningEffortLabel() string {
	if r == nil {
		return ""
	}
	c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
	if err != nil {
		return ""
	}
	return c
}

func (r *Root) SubagentReasoningEffortDisplayLabel() string {
	if lbl := r.SubagentReasoningEffortLabel(); lbl != "" {
		return lbl
	}
	return "none"
}

func (r *Root) EffectiveSubagentReasoningEffort(override string) (canonical string, forceDisable bool) {
	if o := strings.TrimSpace(override); o != "" {
		c, err := ParseReasoningEffortToken(o)
		if err != nil {
			return "none", true
		}
		return c, c == "none"
	}
	if r != nil && strings.TrimSpace(r.SubagentReasoningEffort) != "" {
		c, err := ParseReasoningEffortToken(r.SubagentReasoningEffort)
		if err != nil {
			return "none", true
		}
		return c, c == "none"
	}
	return "none", true
}

func (r *Root) EffectiveFastMode() bool {
	return r == nil || r.FastMode == nil || *r.FastMode
}

func FastModeSupportedByProvider(p *Provider) bool {
	return p != nil && p.IsCursorAPI()
}

func (r *Root) FastModeEnabledForProvider(p *Provider) bool {
	return FastModeSupportedByProvider(p) && r.EffectiveFastMode()
}

func (r *Root) ModelDisplayName(p *Provider, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
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

func NormalizeAnthropicBase(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty base url")
	}
	u, err := url.Parse(raw)
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
	u, err := url.Parse(raw)
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

func NoteRecentModelUse(r *Root, providerName, modelID string) {
	if r == nil {
		return
	}
	prov := strings.TrimSpace(providerName)
	mid := strings.TrimSpace(modelID)
	if prov == "" || mid == "" {
		return
	}
	if r.RecentModels == nil {
		r.RecentModels = make(map[string][]string)
	}
	list := r.RecentModels[prov]
	out := []string{mid}
	for _, m := range list {
		if strings.TrimSpace(m) == mid {
			continue
		}
		out = append(out, m)
		if len(out) >= recentModelUseCap {
			break
		}
	}
	r.RecentModels[prov] = out
}

func RecentModelUseEntries(r *Root, preferProvider string) []RecentModelUse {
	if r == nil || len(r.RecentModels) == 0 {
		return nil
	}
	prefer := strings.TrimSpace(preferProvider)
	names := make([]string, 0, len(r.RecentModels))
	for name := range r.RecentModels {
		names = append(names, name)
	}
	sort.Strings(names)
	var ordered []string
	if prefer != "" {
		ordered = append(ordered, prefer)
		for _, name := range names {
			if name != prefer {
				ordered = append(ordered, name)
			}
		}
	} else {
		ordered = names
	}
	var out []RecentModelUse
	for _, prov := range ordered {
		for _, model := range r.RecentModels[prov] {
			mid := strings.TrimSpace(model)
			if mid == "" {
				continue
			}
			out = append(out, RecentModelUse{Provider: prov, Model: mid})
		}
	}
	return out
}

func ProviderByName(r *Root, name string) *Provider {
	if r == nil || len(r.Providers) == 0 {
		return nil
	}
	return r.Providers[name]
}

func ProviderList(r *Root) []Provider {
	if r == nil || len(r.Providers) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.Providers))
	for name := range r.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Provider, len(names))
	for i, name := range names {
		p := r.Providers[name]
		if p == nil {
			continue
		}
		cp := *p
		cp.Name = name
		out[i] = cp
	}
	return out
}

func FirstProviderName(r *Root) string {
	list := ProviderList(r)
	if len(list) == 0 {
		return ""
	}
	return list[0].Name
}

func IsLocalEndpoint(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return false
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0:0:0:0:0:0:0:1":
		return true
	}
	return strings.HasSuffix(host, ".local")
}

func RemoteProviderNames(r *Root) []string {
	list := ProviderList(r)
	if len(list) == 0 {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, p := range list {
		if !IsLocalEndpoint(p.BaseURL) {
			names = append(names, p.Name)
		}
	}
	return names
}

func (r *Root) WebSearchNeedsInternet() bool {
	if r == nil {
		return true
	}
	engine := strings.ToLower(strings.TrimSpace(r.EffectiveWebSearchEngine()))
	if engine == "searxng" || engine == "searx" {
		if IsLocalEndpoint(r.WebSearchBaseURL) {
			return false
		}
	}
	return true
}

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
