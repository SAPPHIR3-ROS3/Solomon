package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openai/openai-go/v2/shared"
	to "github.com/pelletier/go-toml/v2"
	"solomon/internal/paths"
)

const DefaultSubagentTimeoutMinutes = 20

const DefaultResponseLanguage = "English"

const DefaultCompactionThresholdTokens int64 = 131072

const MinCompactionThresholdTokens int64 = 32768

const DefaultSkillSearchMinNormalizedScore = 0.05

type Provider struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
}

type Current struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

type Root struct {
	Providers                 []Provider `toml:"providers"`
	Current                   Current    `toml:"current"`
	SubagentTimeoutMinutes    int        `toml:"subagent_timeout_minutes"`
	ReasoningEffort           string     `toml:"reasoning_effort"`
	LogLevel                  string     `toml:"log_level"`
	MaxResponseTokens         int        `toml:"max_response_tokens"`
	ShowThinking              bool       `toml:"show_thinking"`
	ShowUsageStats            *bool      `toml:"show_usage_stats"`
	ResponseLanguage          string     `toml:"response_language"`
	CompactionThresholdTokens int64      `toml:"compaction_threshold_tokens"`
	SkillSearchMinNorm        *float64   `toml:"skill_search_min_normalized_score,omitempty"`
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

func RunWizardIfNeeded(stdin io.Reader) (*Root, error) {
	cfgPath, err := paths.ConfigPath()
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		return Load()
	}
	br := bufio.NewScanner(stdin)
	fmt.Print("Solomon setup: OpenAI-compatible API\n")
	fmt.Print("Display name for this provider: ")
	br.Scan()
	name := strings.TrimSpace(br.Text())
	fmt.Print("Base URL (e.g. https://api.openai.com): ")
	br.Scan()
	base := strings.TrimSpace(br.Text())
	fmt.Print("API key: ")
	br.Scan()
	key := strings.TrimSpace(br.Text())
	norm, err := NormalizeAPIBase(base)
	if err != nil {
		return nil, err
	}
	p := Provider{Name: name, BaseURL: norm, APIKey: key}
	fmt.Print("Fetching models…\n")
	mid, err := PickModelInteractive(stdin, &p, name)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Assistant response language [%s]: ", DefaultResponseLanguage)
	br.Scan()
	lang := strings.TrimSpace(br.Text())
	if lang == "" {
		lang = DefaultResponseLanguage
	}
	fmt.Printf("Auto-compact threshold in prompt tokens [%d] (min %d, Enter=default): ", DefaultCompactionThresholdTokens, MinCompactionThresholdTokens)
	br.Scan()
	threshLine := strings.TrimSpace(br.Text())
	var compactionThresh int64 = DefaultCompactionThresholdTokens
	if threshLine != "" {
		tn, err := strconv.ParseInt(threshLine, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("compaction threshold: %w", err)
		}
		if tn < MinCompactionThresholdTokens {
			return nil, fmt.Errorf("compaction threshold must be >= %d", MinCompactionThresholdTokens)
		}
		compactionThresh = tn
	}
	r := &Root{
		Providers:                 []Provider{p},
		Current:                   Current{Provider: name, Model: mid},
		SubagentTimeoutMinutes:    DefaultSubagentTimeoutMinutes,
		ResponseLanguage:          lang,
		CompactionThresholdTokens: compactionThresh,
	}
	if err := Save(r); err != nil {
		return nil, err
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
