package config

import "strings"

type rootLegacyFile struct {
	ProgrammingLanguages      ProgrammingLanguages            `toml:"programming_languages,omitempty"`
	GUI                       GUISettings                     `toml:"gui"`
	UserName                  string                          `toml:"user_name"`
	Providers                 []Provider                      `toml:"providers"`
	Current                   Current                         `toml:"current"`
	RecentModelUses           []RecentModelUse                `toml:"recent_model_uses,omitempty"`
	HiddenModels              map[string][]string             `toml:"hidden_models,omitempty"`
	SubagentTimeoutMinutes    int                             `toml:"subagent_timeout_minutes"`
	SubagentMaxDepth          int                             `toml:"subagent_max_depth"`
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
	LegacyTools               bool                            `toml:"legacy_tools,omitempty"`
	LegacyToolsForce          bool                            `toml:"legacy_tools_force,omitempty"`
	ShowUsageStats            *bool                           `toml:"show_usage_stats"`
	ResponseLanguage          string                          `toml:"response_language"`
	CompactionThresholdTokens int64                           `toml:"compaction_threshold_tokens"`
	Server                    ServerSettings                  `toml:"server,omitempty"`
	ServerPort                int                             `toml:"server_port,omitempty"`
	SkillSearchMinNorm        *float64                        `toml:"skill_search_min_normalized_score,omitempty"`
	DocSearchMinNorm          *float64                        `toml:"doc_search_min_normalized_score,omitempty"`
	DocSearchFullArticleScore *float64                        `toml:"doc_search_full_article_score,omitempty"`
	WebSearchEngine           string                          `toml:"web_search_engine,omitempty"`
	WebSearchAPIKey           string                          `toml:"web_search_api_key,omitempty"`
	WebSearchBaseURL          string                          `toml:"web_search_base_url,omitempty"`
	WebSearchCX               string                          `toml:"web_search_cx,omitempty"`
	ToolOutput                ToolOutput                      `toml:"tool_output,omitempty"`
	APIResilience             APIResilienceConfig             `toml:"api_resilience,omitempty"`
	WebFetch                  WebFetchConfig                  `toml:"web_fetch,omitempty"`
	Export                    Export                          `toml:"export,omitempty"`
	Roles                     Roles                           `toml:"roles,omitempty"`
	AutoUpdate                *bool                           `toml:"autoupdate,omitempty"`
	InstallLayout             string                          `toml:"install_layout,omitempty"`
}

type rootFile struct {
	ProgrammingLanguages      ProgrammingLanguages            `toml:"programming_languages,omitempty"`
	GUI                       GUISettings                     `toml:"gui"`
	UserName                  string                          `toml:"user_name"`
	Providers                 map[string]Provider             `toml:"providers,omitempty"`
	Current                   Current                         `toml:"current"`
	RecentModels              map[string][]string             `toml:"recent_models,omitempty"`
	HiddenModels              map[string][]string             `toml:"hidden_models,omitempty"`
	SubagentTimeoutMinutes    int                             `toml:"subagent_timeout_minutes"`
	SubagentMaxDepth          int                             `toml:"subagent_max_depth"`
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
	LegacyTools               bool                            `toml:"legacy_tools,omitempty"`
	LegacyToolsForce          bool                            `toml:"legacy_tools_force,omitempty"`
	ShowUsageStats            *bool                           `toml:"show_usage_stats"`
	ResponseLanguage          string                          `toml:"response_language"`
	CompactionThresholdTokens int64                           `toml:"compaction_threshold_tokens"`
	Server                    ServerSettings                  `toml:"server,omitempty"`
	ServerPort                int                             `toml:"server_port,omitempty"`
	SkillSearchMinNorm        *float64                        `toml:"skill_search_min_normalized_score,omitempty"`
	DocSearchMinNorm          *float64                        `toml:"doc_search_min_normalized_score,omitempty"`
	DocSearchFullArticleScore *float64                        `toml:"doc_search_full_article_score,omitempty"`
	WebSearchEngine           string                          `toml:"web_search_engine,omitempty"`
	WebSearchAPIKey           string                          `toml:"web_search_api_key,omitempty"`
	WebSearchBaseURL          string                          `toml:"web_search_base_url,omitempty"`
	WebSearchCX               string                          `toml:"web_search_cx,omitempty"`
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
		ProgrammingLanguages:         f.ProgrammingLanguages,
		programmingLanguagesSnapshot: programmingLanguagesFingerprint(f.ProgrammingLanguages),
		UserName:                     f.UserName,
		GUI:                          f.GUI,
		guiSnapshot:                  guiFingerprint(f.GUI),
		Current:                      f.Current,
		RecentModels:                 f.RecentModels,
		HiddenModels:                 f.HiddenModels,
		SubagentTimeoutMinutes:       f.SubagentTimeoutMinutes,
		SubagentMaxDepth:             f.SubagentMaxDepth,
		ReasoningEffort:              f.ReasoningEffort,
		SubagentReasoningEffort:      f.SubagentReasoningEffort,
		FastMode:                     f.FastMode,
		ModelModes:                   f.ModelModes,
		LogLevel:                     f.LogLevel,
		MaxResponseTokens:            f.MaxResponseTokens,
		ShowThinking:                 f.ShowThinking,
		ShowFileChanges:              f.ShowFileChanges,
		Anonymize:                    f.Anonymize,
		Tools:                        mergeToolsSection(f.Tools, f.LegacyTools, f.LegacyToolsForce),
		ShowUsageStats:               f.ShowUsageStats,
		ResponseLanguage:             f.ResponseLanguage,
		CompactionThresholdTokens:    f.CompactionThresholdTokens,
		Server:                       f.Server,
		ServerPort:                   f.ServerPort,
		SkillSearchMinNorm:           f.SkillSearchMinNorm,
		DocSearchMinNorm:             f.DocSearchMinNorm,
		DocSearchFullArticleScore:    f.DocSearchFullArticleScore,
		WebSearchEngine:              f.WebSearchEngine,
		WebSearchAPIKey:              f.WebSearchAPIKey,
		WebSearchBaseURL:             f.WebSearchBaseURL,
		WebSearchCX:                  f.WebSearchCX,
		ToolOutput:                   f.ToolOutput,
		APIResilience:                f.APIResilience,
		WebFetch:                     f.WebFetch,
		Export:                       f.Export,
		Roles:                        f.Roles,
		AutoUpdate:                   f.AutoUpdate,
		InstallLayout:                f.InstallLayout,
		PromptTemplates:              f.PromptTemplates,
		PromptTemplateModTime:        f.PromptTemplateModTime,
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
	if r.GUI.Models.HiddenModels == nil && r.HiddenModels != nil {
		r.GUI.Models.HiddenModels = r.HiddenModels
	}
	f := &rootFile{
		ProgrammingLanguages:      r.ProgrammingLanguages,
		UserName:                  r.UserName,
		GUI:                       r.GUI,
		Current:                   r.Current,
		RecentModels:              r.RecentModels,
		HiddenModels:              nil,
		SubagentTimeoutMinutes:    r.SubagentTimeoutMinutes,
		SubagentMaxDepth:          r.SubagentMaxDepth,
		ReasoningEffort:           r.ReasoningEffort,
		SubagentReasoningEffort:   r.SubagentReasoningEffort,
		FastMode:                  r.FastMode,
		ModelModes:                r.ModelModes,
		LogLevel:                  r.LogLevel,
		MaxResponseTokens:         r.MaxResponseTokens,
		ShowThinking:              r.ShowThinking,
		ShowFileChanges:           r.ShowFileChanges,
		Anonymize:                 r.Anonymize,
		Tools:                     r.Tools,
		ShowUsageStats:            r.ShowUsageStats,
		ResponseLanguage:          r.ResponseLanguage,
		CompactionThresholdTokens: r.CompactionThresholdTokens,
		Server:                    r.Server,
		ServerPort: func() int {
			if r.ServerPort == 0 {
				return DefaultServerPort
			}
			return r.ServerPort
		}(),
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
		Export:                    r.Export,
		Roles:                     r.Roles,
		AutoUpdate:                r.AutoUpdate,
		InstallLayout:             r.InstallLayout,
		PromptTemplates:           r.PromptTemplates,
		PromptTemplateModTime:     r.PromptTemplateModTime,
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
		ProgrammingLanguages:         f.ProgrammingLanguages,
		programmingLanguagesSnapshot: programmingLanguagesFingerprint(f.ProgrammingLanguages),
		UserName:                     f.UserName,
		GUI:                          f.GUI,
		guiSnapshot:                  guiFingerprint(f.GUI),
		Current:                      f.Current,
		HiddenModels:                 f.HiddenModels,
		SubagentTimeoutMinutes:       f.SubagentTimeoutMinutes,
		SubagentMaxDepth:             f.SubagentMaxDepth,
		ReasoningEffort:              f.ReasoningEffort,
		SubagentReasoningEffort:      f.SubagentReasoningEffort,
		FastMode:                     f.FastMode,
		ModelModes:                   f.ModelModes,
		LogLevel:                     f.LogLevel,
		MaxResponseTokens:            f.MaxResponseTokens,
		ShowThinking:                 f.ShowThinking,
		ShowFileChanges:              f.ShowFileChanges,
		Anonymize:                    f.Anonymize,
		Tools:                        mergeToolsSection(f.Tools, f.LegacyTools, f.LegacyToolsForce),
		ShowUsageStats:               f.ShowUsageStats,
		ResponseLanguage:             f.ResponseLanguage,
		CompactionThresholdTokens:    f.CompactionThresholdTokens,
		Server:                       f.Server,
		ServerPort:                   f.ServerPort,
		SkillSearchMinNorm:           f.SkillSearchMinNorm,
		DocSearchMinNorm:             f.DocSearchMinNorm,
		DocSearchFullArticleScore:    f.DocSearchFullArticleScore,
		WebSearchEngine:              f.WebSearchEngine,
		WebSearchAPIKey:              f.WebSearchAPIKey,
		WebSearchBaseURL:             f.WebSearchBaseURL,
		WebSearchCX:                  f.WebSearchCX,
		ToolOutput:                   f.ToolOutput,
		APIResilience:                f.APIResilience,
		WebFetch:                     f.WebFetch,
		Export:                       f.Export,
		Roles:                        f.Roles,
		AutoUpdate:                   f.AutoUpdate,
		InstallLayout:                f.InstallLayout,
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
