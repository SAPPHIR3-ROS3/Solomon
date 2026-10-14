package server

import (
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

// The daemon owns the inhibitor, so closing a client does not release it.
type backgroundAPI struct {
	mu           sync.Mutex
	release      func()
	errorMessage string
	acquire      func() (func(), error)
	save         func(bool) error
}

func newBackgroundAPI() *backgroundAPI {
	a := &backgroundAPI{acquire: acquireSleepInhibitor, save: config.UpdateGUIBackgroundSettings}
	if settings, err := config.ReadGUISettings(); err == nil && settings.Background.KeepAwake {
		a.release, err = a.acquire()
		if err != nil {
			a.errorMessage = err.Error()
			log.Printf("keep awake: %v", err)
		}
	}
	return a
}

func (a *backgroundAPI) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.release != nil {
		a.release()
		a.release = nil
	}
}

func (a *backgroundAPI) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	a.mu.Lock()
	defer a.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
	case http.MethodPatch:
		var request struct {
			KeepAwake *bool `json:"keepAwake"`
		}
		if err := decodeJSONBody(w, r, &request, 1024); err != nil || request.KeepAwake == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("keepAwake must be a boolean"))
			return
		}
		enabled := *request.KeepAwake
		var release func()
		if enabled && a.release == nil {
			var err error
			release, err = a.acquire()
			if err != nil {
				writeAPIError(w, http.StatusServiceUnavailable, err)
				return
			}
		}
		if err := a.save(enabled); err != nil {
			if release != nil {
				release()
			}
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		if release != nil {
			a.release = release
		}
		if !enabled && a.release != nil {
			a.release()
			a.release = nil
		}
		a.errorMessage = ""
	default:
		w.Header().Set("Allow", "GET, PATCH")
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keepAwake": a.release != nil, "error": a.errorMessage})
}
