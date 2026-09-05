package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

type apiActiveAgentProject struct {
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Agents []apiActiveAgentNode `json:"agents"`
}

type apiActiveAgentNode struct {
	Kind             string               `json:"kind"`
	ID               string               `json:"id"`
	Title            string               `json:"title"`
	Status           string               `json:"status"`
	Task             string               `json:"task,omitempty"`
	ProjectID        string               `json:"projectID"`
	ParentChatID     string               `json:"parentChatID,omitempty"`
	ParentToolCallID string               `json:"parentToolCallID,omitempty"`
	RootChatID       string               `json:"rootChatID,omitempty"`
	Children         []apiActiveAgentNode `json:"children,omitempty"`
}

func (a *chatAPI) handleActiveAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	sidebar, err := a.loadProjectSidebarData()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	running := a.runningChatsByProject()
	projects := make([]apiActiveAgentProject, 0, len(sidebar.Projects))
	for _, project := range sidebar.Projects {
		agents := activeAgentsForProject(project, running[project.ID])
		if len(agents) == 0 {
			continue
		}
		projects = append(projects, apiActiveAgentProject{ID: project.ID, Name: project.Name, Agents: agents})
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (a *chatAPI) runningChatsByProject() map[string]map[string]struct{} {
	a.activeMu.Lock()
	defer a.activeMu.Unlock()
	out := make(map[string]map[string]struct{})
	for key := range a.active {
		projectID, chatID, ok := strings.Cut(key, "\x00")
		if !ok || projectID == "" || chatID == "" {
			continue
		}
		chats := out[projectID]
		if chats == nil {
			chats = make(map[string]struct{})
			out[projectID] = chats
		}
		chats[chatID] = struct{}{}
	}
	return out
}

func activeAgentsForProject(project apiProject, running map[string]struct{}) []apiActiveAgentNode {
	subs, err := chatstore.ListSubSessions(project.ID)
	if err != nil {
		subs = nil
	}
	liveSubs := make([]*chatstore.SubSession, 0, len(subs))
	subByID := make(map[string]*chatstore.SubSession)
	for _, sub := range subs {
		if sub == nil || !liveSubagentStatus(sub.Status) {
			continue
		}
		if sub.ProjectHex != "" && sub.ProjectHex != project.ID {
			continue
		}
		liveSubs = append(liveSubs, sub)
		subByID[sub.ID] = sub
	}
	chatTitles := make(map[string]string, len(project.Chats))
	for _, chat := range project.Chats {
		chatTitles[chat.ID] = chat.Title
	}
	shownChats := make(map[string]struct{})
	for chatID := range running {
		shownChats[chatID] = struct{}{}
	}
	for _, sub := range liveSubs {
		rootID := rootChatID(sub.ParentChatID, chatTitles, subByID)
		if rootID != "" {
			shownChats[rootID] = struct{}{}
		}
	}
	children := make(map[string][]apiActiveAgentNode)
	orphans := make([]apiActiveAgentNode, 0)
	for _, sub := range liveSubs {
		node := subagentNode(project.ID, sub, chatTitles, subByID)
		parentID := strings.TrimSpace(sub.ParentChatID)
		if _, isSub := subByID[parentID]; isSub {
			children[parentID] = append(children[parentID], node)
			continue
		}
		if _, isChat := shownChats[parentID]; isChat && parentID != "" {
			children[parentID] = append(children[parentID], node)
			continue
		}
		orphans = append(orphans, node)
	}
	agents := make([]apiActiveAgentNode, 0, len(shownChats)+len(orphans))
	seen := make(map[string]struct{})
	for _, chat := range project.Chats {
		if _, ok := shownChats[chat.ID]; !ok {
			continue
		}
		status := ""
		if _, ok := running[chat.ID]; ok {
			status = "running"
		}
		agents = append(agents, withChildren(apiActiveAgentNode{
			Kind:       "chat",
			ID:         chat.ID,
			Title:      chat.Title,
			Status:     status,
			ProjectID:  project.ID,
			RootChatID: chat.ID,
		}, children))
		seen[chat.ID] = struct{}{}
	}
	for chatID := range shownChats {
		if _, ok := seen[chatID]; ok {
			continue
		}
		status := ""
		if _, ok := running[chatID]; ok {
			status = "running"
		}
		title := chatTitles[chatID]
		if title == "" {
			title = "Untitled chat"
		}
		agents = append(agents, withChildren(apiActiveAgentNode{
			Kind:       "chat",
			ID:         chatID,
			Title:      title,
			Status:     status,
			ProjectID:  project.ID,
			RootChatID: chatID,
		}, children))
	}
	for _, orphan := range orphans {
		agents = append(agents, withChildren(orphan, children))
	}
	return agents
}

func withChildren(node apiActiveAgentNode, children map[string][]apiActiveAgentNode) apiActiveAgentNode {
	direct := children[node.ID]
	if len(direct) == 0 {
		return node
	}
	nested := make([]apiActiveAgentNode, 0, len(direct))
	for _, child := range direct {
		nested = append(nested, withChildren(child, children))
	}
	node.Children = nested
	return node
}

func subagentNode(projectID string, sub *chatstore.SubSession, chatTitles map[string]string, subByID map[string]*chatstore.SubSession) apiActiveAgentNode {
	return apiActiveAgentNode{
		Kind:             "subagent",
		ID:               sub.ID,
		Title:            subSessionTitle(sub),
		Status:           strings.TrimSpace(sub.Status),
		Task:             subSessionTask(sub),
		ProjectID:        projectID,
		ParentChatID:     strings.TrimSpace(sub.ParentChatID),
		ParentToolCallID: strings.TrimSpace(sub.ParentToolCallID),
		RootChatID:       rootChatID(sub.ParentChatID, chatTitles, subByID),
	}
}

func rootChatID(parentID string, chatTitles map[string]string, subByID map[string]*chatstore.SubSession) string {
	seen := make(map[string]struct{})
	current := strings.TrimSpace(parentID)
	for current != "" {
		if _, loop := seen[current]; loop {
			return current
		}
		seen[current] = struct{}{}
		if _, isChat := chatTitles[current]; isChat {
			return current
		}
		sub := subByID[current]
		if sub == nil {
			return current
		}
		current = strings.TrimSpace(sub.ParentChatID)
	}
	return ""
}

func liveSubagentStatus(status string) bool {
	return chatstore.SubSessionRunning(status) || status == chatstore.SubStatusPaused
}

func subSessionTitle(sub *chatstore.SubSession) string {
	if title := strings.TrimSpace(sub.Title); title != "" {
		return title
	}
	if task := subSessionTask(sub); task != "" {
		return task
	}
	return "Subagent"
}

func subSessionTask(sub *chatstore.SubSession) string {
	for _, message := range sub.Messages {
		if message.Role == "user" {
			if task := strings.TrimSpace(message.Content); task != "" {
				return task
			}
		}
	}
	return ""
}
