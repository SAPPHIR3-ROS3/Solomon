package chatstore

import (
	"encoding/json"
	"strconv"
	"strings"
)

type CapabilityRegistry struct {
	DiscoveredTools map[string]json.RawMessage `json:"discovered_tools,omitempty"`
	LoadedSkills    map[string]json.RawMessage `json:"loaded_skills,omitempty"`
	DisabledSkills  map[string]bool            `json:"disabled_skills,omitempty"`
	SDK             json.RawMessage            `json:"sdk,omitempty"`
}

func IsCapabilityTool(name string) bool { return name == "searchTools" || name == "loadSkill" }

func (r *CapabilityRegistry) SetSkillEnabled(name string, enabled bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return
	}
	if !enabled {
		if r.DisabledSkills == nil {
			r.DisabledSkills = map[string]bool{}
		}
		r.DisabledSkills[key] = true
		delete(r.LoadedSkills, key)
		return
	}
	delete(r.DisabledSkills, key)
}

func (r *CapabilityRegistry) Capture(name, payload string) {
	var result map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &result) != nil || result["error"] != nil {
		return
	}
	switch name {
	case "searchTools":
		var tools []json.RawMessage
		if json.Unmarshal(result["tools"], &tools) != nil {
			return
		}
		if r.DiscoveredTools == nil {
			r.DiscoveredTools = map[string]json.RawMessage{}
		}
		for _, tool := range tools {
			var identity struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(tool, &identity) == nil && identity.Name != "" {
				r.DiscoveredTools[identity.Name] = append(json.RawMessage(nil), tool...)
			}
		}
		if sdk := result["sdk"]; len(sdk) > 0 {
			r.SDK = append(json.RawMessage(nil), sdk...)
		}
	case "loadSkill":
		var skill struct {
			Name  string  `json:"name"`
			Slash string  `json:"slash"`
			Body  *string `json:"body"`
		}
		if json.Unmarshal([]byte(payload), &skill) != nil || skill.Body == nil {
			return
		}
		key := strings.ToLower(strings.TrimSpace(skill.Name))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(skill.Slash))
		}
		if key == "" {
			return
		}
		if r.DisabledSkills[key] {
			return
		}
		if r.LoadedSkills == nil {
			r.LoadedSkills = map[string]json.RawMessage{}
		}
		r.LoadedSkills[key] = json.RawMessage(payload)
	}
}

func skillPayload(content string) (name, body, request string, ok bool) {
	const prefix = "Apply the agent skill "
	if strings.HasPrefix(content, prefix) {
		parts := strings.SplitN(strings.TrimPrefix(content, prefix), ":\n\n", 2)
		if len(parts) == 2 {
			name, _ = strconv.Unquote(parts[0])
			return name, parts[1], "Apply the agent skill " + parts[0] + ".", name != ""
		}
	}
	const forced = "You must use the following installed skill for this turn.\n\nSkill: "
	if strings.HasPrefix(content, forced) {
		parts := strings.SplitN(strings.TrimPrefix(content, forced), "\n\n--- SKILL START ---\n", 2)
		if len(parts) == 2 {
			name, _ = strconv.Unquote(parts[0])
			end := strings.LastIndex(parts[1], "\n--- SKILL END ---\n\n")
			if end >= 0 {
				request = strings.TrimPrefix(parts[1][end+len("\n--- SKILL END ---\n\n"):], "User request:\n")
				return name, parts[1][:end], request, name != ""
			}
		}
	}
	return "", "", "", false
}

func (r *CapabilityRegistry) Backfill(messages []Message) {
	var recovered CapabilityRegistry
	calls := map[string]string{}
	for _, m := range messages {
		for _, tc := range m.ToolCalls {
			calls[tc.ID] = tc.Name
		}
		if m.Role == "tool" {
			recovered.Capture(calls[m.ToolCallID], m.Content)
		}
		if m.Role == "user" {
			if strings.HasPrefix(m.Content, "tool_result(") && strings.HasSuffix(m.Content, ")") {
				payload := strings.TrimSuffix(strings.TrimPrefix(m.Content, "tool_result("), ")")
				recovered.Capture("searchTools", payload)
				recovered.Capture("loadSkill", payload)
			}
			recovered.CaptureUser(m)
		}
	}
	if r.DiscoveredTools == nil {
		r.DiscoveredTools = map[string]json.RawMessage{}
	}
	for key, tool := range recovered.DiscoveredTools {
		if _, exists := r.DiscoveredTools[key]; !exists {
			r.DiscoveredTools[key] = tool
		}
	}
	if r.LoadedSkills == nil {
		r.LoadedSkills = map[string]json.RawMessage{}
	}
	for key, skill := range recovered.LoadedSkills {
		if r.DisabledSkills[key] {
			continue
		}
		if _, exists := r.LoadedSkills[key]; !exists {
			r.LoadedSkills[key] = skill
		}
	}
	if len(r.SDK) == 0 {
		r.SDK = recovered.SDK
	}
}

func (r CapabilityRegistry) RestoreMessage() (Message, bool) {
	if len(r.DiscoveredTools) == 0 && len(r.LoadedSkills) == 0 && len(r.DisabledSkills) == 0 && len(r.SDK) == 0 {
		return Message{}, false
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return Message{}, false
	}
	return Message{Role: "user", GUIHidden: true, ContextKind: "capabilities", Content: "[Restored capabilities]\n" + string(payload)}, true
}

func SummaryMessages(messages []Message) []Message {
	calls := map[string]string{}
	for _, m := range messages {
		for _, tc := range m.ToolCalls {
			calls[tc.ID] = tc.Name
		}
	}
	var out []Message
	for _, m := range messages {
		if m.Role == "system" || m.Role == "developer" || m.ContextKind == "capabilities" {
			continue
		}
		if m.Role == "tool" && IsCapabilityTool(calls[m.ToolCallID]) {
			continue
		}
		if m.Role == "user" {
			if strings.HasPrefix(m.Content, "tool_result(") {
				payload := strings.TrimSuffix(strings.TrimPrefix(m.Content, "tool_result("), ")")
				var result map[string]json.RawMessage
				if json.Unmarshal([]byte(payload), &result) == nil && (result["tools"] != nil || (result["body"] != nil && result["name"] != nil)) {
					continue
				}
			}
			content := m.APIContent
			if content == "" {
				content = m.Content
			}
			if _, _, request, ok := skillPayload(content); ok {
				if m.APIContent == "" {
					m.Content = request
				}
				m.APIContent = ""
			}
		}
		m.ToolCalls = append([]ToolCall(nil), m.ToolCalls...)
		kept := m.ToolCalls[:0]
		for _, tc := range m.ToolCalls {
			if !IsCapabilityTool(tc.Name) {
				kept = append(kept, tc)
			}
		}
		m.ToolCalls = kept
		if m.Role == "assistant" && m.Content == "" && len(kept) == 0 {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (r *CapabilityRegistry) CaptureUser(m Message) {
	if m.Role != "user" {
		return
	}
	content := m.APIContent
	if content == "" {
		content = m.Content
	}
	if name, body, _, ok := skillPayload(content); ok {
		payload, err := json.Marshal(map[string]string{"name": name, "body": body})
		if err == nil {
			r.Capture("loadSkill", string(payload))
		}
	}
}
