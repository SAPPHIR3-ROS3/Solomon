package config



import (

	"bytes"

	"os"

	"path/filepath"

	"strings"



	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"

	to "github.com/pelletier/go-toml/v2"

)



type rootLegacyFile struct {

	UserName                  string           `toml:"user_name"`

	Providers                 []Provider       `toml:"providers"`

	Current                   Current          `toml:"current"`

	RecentModelUses           []RecentModelUse `toml:"recent_model_uses,omitempty"`

	SubagentTimeoutMinutes    int              `toml:"subagent_timeout_minutes"`

	ReasoningEffort           string           `toml:"reasoning_effort"`

	SubagentReasoningEffort     string           `toml:"subagent_reasoning_effort"`

	FastMode                  *bool            `toml:"fast_mode,omitempty"`

	LogLevel                  string           `toml:"log_level"`

	MaxResponseTokens         int              `toml:"max_response_tokens"`

	ShowThinking              bool             `toml:"show_thinking"`

	Anonymize                 bool             `toml:"anonymize,omitempty"`

	Tools                     Tools            `toml:"tools,omitempty"`

	LegacyTools               bool             `toml:"legacy_tools,omitempty"`

	LegacyToolsForce          bool             `toml:"legacy_tools_force,omitempty"`

	ShowUsageStats            *bool            `toml:"show_usage_stats"`

	ResponseLanguage          string           `toml:"response_language"`

	CompactionThresholdTokens int64            `toml:"compaction_threshold_tokens"`

	SkillSearchMinNorm        *float64         `toml:"skill_search_min_normalized_score,omitempty"`

	DocSearchMinNorm          *float64         `toml:"doc_search_min_normalized_score,omitempty"`

	DocSearchFullArticleScore *float64         `toml:"doc_search_full_article_score,omitempty"`

	WebSearchEngine           string           `toml:"web_search_engine,omitempty"`

	WebSearchAPIKey           string           `toml:"web_search_api_key,omitempty"`

	WebSearchBaseURL          string           `toml:"web_search_base_url,omitempty"`

	WebSearchCX               string           `toml:"web_search_cx,omitempty"`

	ToolOutput                ToolOutput            `toml:"tool_output,omitempty"`

	APIResilience             APIResilienceConfig   `toml:"api_resilience,omitempty"`

	WebFetch                  WebFetchConfig        `toml:"web_fetch,omitempty"`

	AutoUpdate                *bool                 `toml:"autoupdate,omitempty"`

}



type rootFile struct {

	UserName                  string              `toml:"user_name"`

	Providers                 map[string]Provider `toml:"providers,omitempty"`

	Current                   Current             `toml:"current"`

	RecentModels              map[string][]string `toml:"recent_models,omitempty"`

	SubagentTimeoutMinutes    int                 `toml:"subagent_timeout_minutes"`

	ReasoningEffort           string              `toml:"reasoning_effort"`

	SubagentReasoningEffort   string              `toml:"subagent_reasoning_effort"`

	FastMode                  *bool               `toml:"fast_mode,omitempty"`

	LogLevel                  string              `toml:"log_level"`

	MaxResponseTokens         int                 `toml:"max_response_tokens"`

	ShowThinking              bool                `toml:"show_thinking"`

	Anonymize                 bool                `toml:"anonymize,omitempty"`

	Tools                     Tools               `toml:"tools,omitempty"`

	LegacyTools               bool                `toml:"legacy_tools,omitempty"`

	LegacyToolsForce          bool                `toml:"legacy_tools_force,omitempty"`

	ShowUsageStats            *bool               `toml:"show_usage_stats"`

	ResponseLanguage          string              `toml:"response_language"`

	CompactionThresholdTokens int64               `toml:"compaction_threshold_tokens"`

	SkillSearchMinNorm        *float64            `toml:"skill_search_min_normalized_score,omitempty"`

	DocSearchMinNorm          *float64            `toml:"doc_search_min_normalized_score,omitempty"`

	DocSearchFullArticleScore *float64            `toml:"doc_search_full_article_score,omitempty"`

	WebSearchEngine           string              `toml:"web_search_engine,omitempty"`

	WebSearchAPIKey           string              `toml:"web_search_api_key,omitempty"`

	WebSearchBaseURL          string              `toml:"web_search_base_url,omitempty"`

	WebSearchCX               string              `toml:"web_search_cx,omitempty"`

	ToolOutput                ToolOutput            `toml:"tool_output,omitempty"`

	APIResilience             APIResilienceConfig   `toml:"api_resilience,omitempty"`

	WebFetch                  WebFetchConfig        `toml:"web_fetch,omitempty"`

	AutoUpdate                *bool                 `toml:"autoupdate,omitempty"`

	PromptTemplates           map[string]string     `toml:"prompt_templates,omitempty"`

	PromptTemplateModTime   map[string]int64 `toml:"prompt_template_mtime,omitempty"`

}



func mergeToolsSection(section Tools, legacyRoot, legacyForceRoot bool) Tools {

	if legacyRoot {

		section.Legacy = true

	}

	if legacyForceRoot {

		section.LegacyForce = true

	}

	return section

}



func rootFromFile(f *rootFile) *Root {

	if f == nil {

		return EmptyRoot()

	}

	r := &Root{

		UserName:                  f.UserName,

		Current:                   f.Current,

		RecentModels:              f.RecentModels,

		SubagentTimeoutMinutes:    f.SubagentTimeoutMinutes,

		ReasoningEffort:           f.ReasoningEffort,

		SubagentReasoningEffort:   f.SubagentReasoningEffort,

		FastMode:                  f.FastMode,

		LogLevel:                  f.LogLevel,

		MaxResponseTokens:         f.MaxResponseTokens,

		ShowThinking:              f.ShowThinking,

		Anonymize:                 f.Anonymize,

		Tools:                     mergeToolsSection(f.Tools, f.LegacyTools, f.LegacyToolsForce),

		ShowUsageStats:            f.ShowUsageStats,

		ResponseLanguage:          f.ResponseLanguage,

		CompactionThresholdTokens: f.CompactionThresholdTokens,

		SkillSearchMinNorm:        f.SkillSearchMinNorm,

		DocSearchMinNorm:          f.DocSearchMinNorm,

		DocSearchFullArticleScore: f.DocSearchFullArticleScore,

		WebSearchEngine:           f.WebSearchEngine,

		WebSearchAPIKey:           f.WebSearchAPIKey,

		WebSearchBaseURL:          f.WebSearchBaseURL,

		WebSearchCX:               f.WebSearchCX,

		ToolOutput:                f.ToolOutput,

		APIResilience:             f.APIResilience,

		WebFetch:                  f.WebFetch,

		AutoUpdate:                f.AutoUpdate,

		PromptTemplates:           f.PromptTemplates,

		PromptTemplateModTime:   f.PromptTemplateModTime,

	}

	for name, p := range f.Providers {

		setProviderOnRoot(r, name, p)

	}

	return r

}



func rootToFile(r *Root) *rootFile {

	if r == nil {

		return &rootFile{}

	}

	f := &rootFile{

		UserName:                  r.UserName,

		Current:                   r.Current,

		RecentModels:              r.RecentModels,

		SubagentTimeoutMinutes:    r.SubagentTimeoutMinutes,

		ReasoningEffort:           r.ReasoningEffort,

		SubagentReasoningEffort:   r.SubagentReasoningEffort,

		FastMode:                  r.FastMode,

		LogLevel:                  r.LogLevel,

		MaxResponseTokens:         r.MaxResponseTokens,

		ShowThinking:              r.ShowThinking,

		Anonymize:                 r.Anonymize,

		Tools:                     r.Tools,

		ShowUsageStats:            r.ShowUsageStats,

		ResponseLanguage:          r.ResponseLanguage,

		CompactionThresholdTokens: r.CompactionThresholdTokens,

		SkillSearchMinNorm:        r.SkillSearchMinNorm,

		DocSearchMinNorm:          r.DocSearchMinNorm,

		DocSearchFullArticleScore: r.DocSearchFullArticleScore,

		WebSearchEngine:           r.WebSearchEngine,

		WebSearchAPIKey:           r.WebSearchAPIKey,

		WebSearchBaseURL:          r.WebSearchBaseURL,

		WebSearchCX:               r.WebSearchCX,

		ToolOutput:                r.ToolOutput,

		APIResilience:             r.APIResilience,

		WebFetch:                  r.WebFetch,

		AutoUpdate:                r.AutoUpdate,

		PromptTemplates:           r.PromptTemplates,

		PromptTemplateModTime:   r.PromptTemplateModTime,

	}

	if len(r.Providers) > 0 {

		f.Providers = make(map[string]Provider, len(r.Providers))

		for name, p := range r.Providers {

			if p == nil {

				continue

			}

			cp := *p

			cp.Name = name

			f.Providers[name] = cp

		}

	}

	return f

}



func setProviderOnRoot(r *Root, name string, p Provider) {

	name = strings.TrimSpace(name)

	if r == nil || name == "" {

		return

	}

	if r.Providers == nil {

		r.Providers = make(map[string]*Provider)

	}

	cp := p

	cp.Name = name

	r.Providers[name] = &cp

}



func rootFromLegacy(f *rootLegacyFile) *Root {

	if f == nil {

		return EmptyRoot()

	}

	r := &Root{

		UserName:                  f.UserName,

		Current:                   f.Current,

		SubagentTimeoutMinutes:    f.SubagentTimeoutMinutes,

		ReasoningEffort:           f.ReasoningEffort,

		SubagentReasoningEffort:   f.SubagentReasoningEffort,

		FastMode:                  f.FastMode,

		LogLevel:                  f.LogLevel,

		MaxResponseTokens:         f.MaxResponseTokens,

		ShowThinking:              f.ShowThinking,

		Anonymize:                 f.Anonymize,

		Tools:                     mergeToolsSection(f.Tools, f.LegacyTools, f.LegacyToolsForce),

		ShowUsageStats:            f.ShowUsageStats,

		ResponseLanguage:          f.ResponseLanguage,

		CompactionThresholdTokens: f.CompactionThresholdTokens,

		SkillSearchMinNorm:        f.SkillSearchMinNorm,

		DocSearchMinNorm:          f.DocSearchMinNorm,

		DocSearchFullArticleScore: f.DocSearchFullArticleScore,

		WebSearchEngine:           f.WebSearchEngine,

		WebSearchAPIKey:           f.WebSearchAPIKey,

		WebSearchBaseURL:          f.WebSearchBaseURL,

		WebSearchCX:               f.WebSearchCX,

		ToolOutput:                f.ToolOutput,

		APIResilience:             f.APIResilience,

		WebFetch:                  f.WebFetch,

		AutoUpdate:                f.AutoUpdate,

	}

	for _, p := range f.Providers {

		name := strings.TrimSpace(p.Name)

		if name == "" {

			continue

		}

		setProviderOnRoot(r, name, p)

	}

	for i := len(f.RecentModelUses) - 1; i >= 0; i-- {

		u := f.RecentModelUses[i]

		NoteRecentModelUse(r, u.Provider, u.Model)

	}

	return r

}



func normalizeRoot(r *Root) {

	if r == nil {

		return

	}

	for name, p := range r.Providers {

		if p == nil {

			delete(r.Providers, name)

			continue

		}

		if strings.TrimSpace(p.Name) == "" {

			p.Name = name

		}

	}

}



func Load() (*Root, error) {

	cfgPath, err := paths.ConfigPath()

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config path resolve failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return nil, err

	}

	b, err := os.ReadFile(cfgPath)

	if err != nil {

		if !os.IsNotExist(err) {

			logging.Log(logging.ERROR_LOG_LEVEL, "config read failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		}

		return nil, err

	}

	if bytes.Contains(b, []byte("[[providers]]")) {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err != nil {

			logging.Log(logging.ERROR_LOG_LEVEL, "config unmarshal legacy failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

			return nil, err

		}

		r := rootFromLegacy(&leg)

		normalizeRoot(r)

		return r, nil

	}

	var f rootFile

	if err := to.Unmarshal(b, &f); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config unmarshal failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		return nil, err

	}

	r := rootFromFile(&f)

	if len(r.Providers) == 0 {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err == nil && len(leg.Providers) > 0 {

			r = rootFromLegacy(&leg)

		}

	}

	if len(r.RecentModels) == 0 && bytes.Contains(b, []byte("[[recent_model_uses]]")) {

		var leg rootLegacyFile

		if err := to.Unmarshal(b, &leg); err == nil {

			for i := len(leg.RecentModelUses) - 1; i >= 0; i-- {

				u := leg.RecentModelUses[i]

				NoteRecentModelUse(r, u.Provider, u.Model)

			}

		}

	}

	normalizeRoot(r)

	return r, nil

}



func Save(r *Root) error {

	cfgPath, err := paths.ConfigPath()

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config path resolve failed on save", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return err

	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config mkdir failed", logging.LogOptions{Params: map[string]any{"path": filepath.Dir(cfgPath), "err": err.Error()}})

		return err

	}

	buf, err := to.Marshal(rootToFile(r))

	if err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config marshal failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})

		return err

	}

	tmp := cfgPath + ".tmp"

	if err := os.WriteFile(tmp, buf, 0o600); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config write temp failed", logging.LogOptions{Params: map[string]any{"path": tmp, "err": err.Error()}})

		return err

	}

	if err := os.Rename(tmp, cfgPath); err != nil {

		logging.Log(logging.ERROR_LOG_LEVEL, "config rename failed", logging.LogOptions{Params: map[string]any{"path": cfgPath, "err": err.Error()}})

		return err

	}

	return nil

}


