package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
	"github.com/openai/openai-go/v2/shared"
	to "github.com/pelletier/go-toml/v2"
)

const DefaultSubagentTimeoutMinutes = 20

const DefaultResponseLanguage = "English"

const DefaultCompactionThresholdTokens int64 = 131072

const MinCompactionThresholdTokens int64 = 32768

const DefaultSkillSearchMinNormalizedScore = 0.05

const DefaultWebSearchEngine = "duckduckgo"

type Provider struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
}

type Current struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

type RecentModelUse struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

type Root struct {
	UserName                  string     `toml:"user_name"`
	Providers                 []Provider `toml:"providers"`
	Current                   Current          `toml:"current"`
	RecentModelUses           []RecentModelUse `toml:"recent_model_uses,omitempty"`
	SubagentTimeoutMinutes    int              `toml:"subagent_timeout_minutes"`
	ReasoningEffort           string     `toml:"reasoning_effort"`
	LogLevel                  string     `toml:"log_level"`
	MaxResponseTokens         int        `toml:"max_response_tokens"`
	ShowThinking              bool       `toml:"show_thinking"`
	ShowUsageStats            *bool      `toml:"show_usage_stats"`
	ResponseLanguage          string     `toml:"response_language"`
	CompactionThresholdTokens int64      `toml:"compaction_threshold_tokens"`
	SkillSearchMinNorm        *float64   `toml:"skill_search_min_normalized_score,omitempty"`
	WebSearchEngine           string     `toml:"web_search_engine,omitempty"`
	WebSearchAPIKey           string     `toml:"web_search_api_key,omitempty"`
	WebSearchBaseURL          string     `toml:"web_search_base_url,omitempty"`
	WebSearchCX               string     `toml:"web_search_cx,omitempty"`
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

func Load() (*Root, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var r Root
	if err := to.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func Save(r *Root) error {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	buf, err := to.Marshal(r)
	if err != nil {
		return err
	}
	tmp := cfgPath + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, cfgPath)
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
	use := RecentModelUse{Provider: prov, Model: mid}
	out := make([]RecentModelUse, 0, len(r.RecentModelUses)+1)
	out = append(out, use)
	for _, x := range r.RecentModelUses {
		if strings.TrimSpace(x.Provider) == prov && strings.TrimSpace(x.Model) == mid {
			continue
		}
		out = append(out, x)
		if len(out) >= recentModelUseCap {
			break
		}
	}
	r.RecentModelUses = out
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
	for i := range r.Providers {
		if r.Providers[i].Name == name {
			p := &r.Providers[i]
			return p, nil
		}
	}
	if len(r.Providers) > 0 {
		p := &r.Providers[0]
		r.Current.Provider = p.Name
		return p, Save(r)
	}
	return nil, errors.New("no providers in config")
}
