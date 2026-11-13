package server

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func (a *customizationAPI) addSkill(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Source string `json:"source"`
		Name   string `json:"name"`
	}
	if err := decodeJSONBody(w, r, &request, 16<<10); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	source := strings.TrimSpace(request.Source)
	if !skills.IsSkillMarkdownSource(source) && !skills.IsSkillsShURL(source) {
		writeAPIError(w, http.StatusBadRequest, errors.New("provide a .md path, .md URL, or skills.sh URL"))
		return
	}
	args := []string{"skill", source}
	if name := strings.TrimSpace(request.Name); name != "" {
		args = append(args, name)
	}
	args = append(args, skills.ScopeGlobal)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := skills.RunInstall(skills.InstallOpts{
		Ctx: r.Context(), Args: args, NonInteractive: true,
		In: strings.NewReader(""), Out: io.Discard,
	}); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	items, err := readGlobalSkills()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": items})
}

func (a *customizationAPI) editSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, errors.New("skill id is required"))
		return
	}
	var content *string
	if r.Method == http.MethodPatch {
		var request struct {
			Content *string `json:"content"`
		}
		if err := decodeJSONBody(w, r, &request, 5<<20); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Content == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("content is required"))
			return
		}
		content = request.Content
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	markdown, err := skills.GlobalMarkdown(id, content, r.Method == http.MethodDelete)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]string{"content": markdown})
		return
	}
	items, err := readGlobalSkills()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": items})
}
