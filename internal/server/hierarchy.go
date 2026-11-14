package server

import (
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func subagentHierarchy(projectID string, session *chatstore.SubSession) (int, string) {
	if session.Depth > 0 && (session.Depth == 1 || session.ParentSubchatID != "") {
		return session.Depth, session.ParentSubchatID
	}
	sessions, err := chatstore.ListSubSessions(projectID)
	if err != nil {
		return 1, session.ParentSubchatID
	}
	byID := make(map[string]*chatstore.SubSession, len(sessions))
	parents := make(map[string]string)
	for _, candidate := range sessions {
		byID[candidate.ID] = candidate
		if candidate.ParentSubchatID != "" {
			parents[candidate.ID] = candidate.ParentSubchatID
		}
	}
	for _, candidate := range sessions {
		if _, exists := byID[candidate.ParentChatID]; exists && parents[candidate.ID] == "" {
			parents[candidate.ID] = candidate.ParentChatID
		}
		for _, message := range candidate.Messages {
			if message.Role != "tool" {
				continue
			}
			var result struct {
				SubchatID string `json:"subchatId"`
			}
			if json.Unmarshal([]byte(message.Content), &result) != nil || result.SubchatID == candidate.ID {
				continue
			}
			if _, exists := byID[result.SubchatID]; exists && parents[result.SubchatID] == "" {
				parents[result.SubchatID] = candidate.ID
			}
		}
	}
	seen := make(map[string]bool)
	var depth func(string) int
	depth = func(id string) int {
		if seen[id] {
			return 1
		}
		seen[id] = true
		if candidate := byID[id]; candidate != nil && candidate.Depth > 0 {
			return candidate.Depth
		}
		if parent := parents[id]; parent != "" {
			return depth(parent) + 1
		}
		return 1
	}
	return depth(session.ID), parents[session.ID]
}
