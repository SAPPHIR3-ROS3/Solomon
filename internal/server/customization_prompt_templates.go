package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
)

func (a *customizationAPI) handlePromptTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	items, err := loadPromptTemplates()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplates": items})
}

func (a *customizationAPI) handlePromptTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	item, err := loadPromptTemplate(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplate": item})
}

func (a *customizationAPI) handleUpdatePromptTemplate(w http.ResponseWriter, r *http.Request) {
	a.handlePromptTemplateMutation(w, r, false)
}

func (a *customizationAPI) handleResetPromptTemplate(w http.ResponseWriter, r *http.Request) {
	a.handlePromptTemplateMutation(w, r, true)
}

func (a *customizationAPI) handlePromptTemplateMutation(w http.ResponseWriter, r *http.Request, reset bool) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Content string `json:"content"`
		ID      string `json:"id"`
	}
	if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var err error
	if reset {
		_, err = prompt.ResetTemplateToDefault(strings.TrimSpace(request.ID))
	} else {
		err = prompt.AcceptTemplateContent(strings.TrimSpace(request.ID), request.Content)
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	item, err := loadPromptTemplate(request.ID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"promptTemplate": item})
}

func loadPromptTemplates() ([]apiCustomizationItem, error) {
	names := prompt.TemplateNames()
	sort.Strings(names)
	items := make([]apiCustomizationItem, 0, len(names))
	for _, name := range names {
		item := apiCustomizationItem{Detail: promptTemplateDetail(name), ID: name, Title: name + ".tmpl"}
		directory, err := prompt.TemplatesDir()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(directory, item.Title)); errors.Is(err, os.ErrNotExist) {
			item.Badge = "Missing"
		} else if modified, diffErr := prompt.TemplateDiffersFromEmbedded(name); diffErr == nil && modified {
			item.Badge = "Modified"
		}
		items = append(items, item)
	}
	return items, nil
}

func loadPromptTemplate(id string) (apiPromptTemplate, error) {
	id = strings.TrimSpace(id)
	if _, ok := prompt.EmbeddedTemplate(id); !ok {
		return apiPromptTemplate{}, errors.New("unknown prompt template")
	}
	content, err := prompt.TemplateContent(id)
	if err != nil {
		return apiPromptTemplate{}, err
	}
	modified, err := prompt.TemplateDiffersFromEmbedded(id)
	if err != nil {
		return apiPromptTemplate{}, err
	}
	return apiPromptTemplate{Content: content, ID: id, Modified: modified, Title: id + ".tmpl"}, nil
}

func promptTemplateDetail(name string) string {
	switch name {
	case "agent":
		return "Main agent-mode system prompt"
	case "atmention":
		return "At-mention workflow prompt"
	case "btw":
		return "Side-question (/btw) prompt"
	case "btw_system":
		return "Side-question (/btw) system prompt"
	case "chat":
		return "Chat-mode system prompt"
	case "images":
		return "Image workflow prompt"
	case "summarize":
		return "Conversation summarize prompt"
	case "summarize_system":
		return "Conversation summarize system prompt"
	case "title":
		return "Chat title generation prompt"
	default:
		return "System prompt template"
	}
}
