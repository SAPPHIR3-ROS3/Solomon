package agentruntime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func nestedRunConfigFromToolArgs(r *Runtime, args map[string]json.RawMessage) (NestedRunConfig, error) {
	cfg := NestedRunConfig{
		SpawnTime: time.Now().UTC(),
		Origin:    chatstore.SubOriginParent,
		ProjectHex: r.ProjHex,
	}
	if r.Session != nil {
		cfg.ParentChatID = r.Session.ID
	}
	if v := jsonString(args["sysPromptPath"]); v != "" {
		cfg.SysPromptPath = v
	}
	if v := jsonString(args["task"]); v != "" {
		cfg.Task = v
	}
	if v := jsonString(args["resume"]); v != "" {
		cfg.ResumeID = v
	}
	if b, ok := jsonBool(args["run_in_background"]); ok {
		cfg.RunInBackground = b
	}
	if v := jsonString(args["reasoningEffort"]); v != "" {
		cfg.ReasoningEffort = v
	}
	if v := jsonString(args["roleProvider"]); v != "" {
		cfg.RoleProvider = v
	}
	if v := jsonString(args["roleModel"]); v != "" {
		cfg.RoleModel = v
	}
	cfg.ToolCall = chatstore.ToolCall{
		Name:      "subagent",
		Arguments: string(mustMarshal(args)),
	}
	return cfg, nil
}

func nestedRunConfigFromExec(parentChatID, parentToolCallID string, projectHex string, tc chatstore.ToolCall, raw json.RawMessage) (NestedRunConfig, error) {
	var a struct {
		SysPromptPath   string `json:"sysPromptPath"`
		Task            string `json:"task"`
		Resume          string `json:"resume"`
		RunInBackground bool   `json:"run_in_background"`
		ReasoningEffort string `json:"reasoningEffort"`
		RoleProvider    string `json:"roleProvider"`
		RoleModel       string `json:"roleModel"`
	}
	_ = json.Unmarshal(raw, &a)
	cfg := NestedRunConfig{
		SysPromptPath:    a.SysPromptPath,
		Task:             a.Task,
		ResumeID:         a.Resume,
		RunInBackground:  a.RunInBackground,
		ReasoningEffort:  a.ReasoningEffort,
		RoleProvider:     strings.TrimSpace(a.RoleProvider),
		RoleModel:        strings.TrimSpace(a.RoleModel),
		ParentChatID:     parentChatID,
		ParentToolCallID: parentToolCallID,
		ToolCall:         tc,
		SpawnTime:        time.Now().UTC(),
		Origin:           chatstore.SubOriginParent,
		ProjectHex:       projectHex,
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) == nil {
		if b, ok := jsonBool(m["run_in_background"]); ok {
			cfg.RunInBackground = b
		}
	}
	if cfg.ToolCall.Name == "" {
		cfg.ToolCall = chatstore.ToolCall{Name: "subagent", Arguments: string(raw)}
	}
	return cfg, nil
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func jsonBool(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b, true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
