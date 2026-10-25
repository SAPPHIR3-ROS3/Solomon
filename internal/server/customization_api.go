package server

import (
	"errors"
	"net/http"
	"os"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

// customizationAPI is deliberately daemon-owned. The mutex serializes file
// rewrites so a rule reorder cannot race a simultaneous edit from another
// client (for example a browser and the Wails window).
type customizationAPI struct {
	mu sync.Mutex
}

type apiCustomizationScore struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

type apiCustomizationItem struct {
	Disabled bool                    `json:"disabled,omitempty"`
	Badge    string                  `json:"badge,omitempty"`
	Detail   string                  `json:"detail"`
	ID       string                  `json:"id"`
	Scores   []apiCustomizationScore `json:"scores,omitempty"`
	Title    string                  `json:"title"`
}

type apiRolesCharacteristic struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type apiRolesTable struct {
	Catalog         []apiRolesCharacteristic `json:"catalog"`
	Characteristics []string                 `json:"characteristics"`
	Max             int                      `json:"max"`
}

type apiPromptTemplate struct {
	Content  string `json:"content"`
	ID       string `json:"id"`
	Modified bool   `json:"modified"`
	Title    string `json:"title"`
}

func newCustomizationAPI() *customizationAPI { return &customizationAPI{} }

func (a *customizationAPI) handleGlobalAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var request struct {
		Content *string `json:"content"`
	}
	if r.Method == http.MethodPost {
		if err := decodeJSONBody(w, r, &request, 256<<10); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Content == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("content is required"))
			return
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := paths.EnsureGlobalAgentsPath()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if request.Content != nil {
		if err := os.WriteFile(p, []byte(*request.Content), 0o600); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
	}
	content, err := os.ReadFile(p)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"globalAgents": map[string]string{"path": p, "content": string(content)}})
}

func (a *customizationAPI) handleCatalogGET(w http.ResponseWriter, r *http.Request, key string, reader func() ([]apiCustomizationItem, error)) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	items, err := reader()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{key: items})
}
