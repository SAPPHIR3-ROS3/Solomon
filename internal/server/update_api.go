package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

type updateStatus struct {
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Phase      string `json:"phase"`
	Error      string `json:"error,omitempty"`
	Notice     string `json:"notice,omitempty"`
}

type updateAPI struct {
	mu             sync.Mutex
	ctx            context.Context
	status         updateStatus
	checked        time.Time
	closed         bool
	staged, target string
	check          func(context.Context) updater.CheckResult
	prepare        func(context.Context, string, io.Writer) (string, string, error)
	launch         func(string, string, string) error
}

func newUpdateAPI(ctx context.Context, build State) *updateAPI {
	status := updateStatus{Current: build.Version, Phase: "idle"}
	if updater.IsDevelopmentVersion(build.Version) {
		status.Notice = "Local development build. Updating installs the official release; local source files are kept."
	}
	return &updateAPI{
		ctx: ctx, status: status,
		check: func(ctx context.Context) updater.CheckResult {
			// A local dev suffix alone is not an update. Offer only a release
			// newer than the build baseline; installation remains explicit.
			return updater.CheckForNewerRelease(ctx, build.Version)
		},
		prepare: updater.PrepareInstall, launch: updater.LaunchPreparedUpdate,
	}
}

func (a *updateAPI) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	if a.status.Phase != "restarting" && a.staged != "" {
		_ = os.Remove(a.staged)
		_ = os.Remove(a.staged + ".error")
	}
}

func (a *updateAPI) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	if r.Method == http.MethodPost && !terminalOriginAllowed(r) {
		writeAPIError(w, http.StatusForbidden, errors.New("request origin is not allowed"))
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.Method == http.MethodGet {
		if a.status.Phase == "restarting" {
			if message, err := os.ReadFile(a.staged + ".error"); err == nil {
				a.status.Phase, a.status.Error = "available", string(message)
				_ = os.Remove(a.staged)
				_ = os.Remove(a.staged + ".error")
				a.staged, a.target = "", ""
				a.checked = time.Now()
			}
		}
		if (a.status.Phase == "idle" || a.status.Phase == "available") && time.Since(a.checked) >= 5*time.Minute {
			a.status.Phase = "checking"
			go a.refresh()
		}
		writeJSON(w, http.StatusOK, a.status)
		return
	}
	var request struct {
		Action string `json:"action"`
	}
	if err := decodeJSONBody(w, r, &request, 1024); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	switch request.Action {
	case "download":
		if a.status.Phase != "available" {
			writeAPIError(w, http.StatusConflict, errors.New("no update available to download"))
			return
		}
		a.status.Phase, a.status.Error = "downloading", ""
		a.status.Downloaded, a.status.Total = 0, 0
		go a.download(a.status.Latest)
	case "install":
		if a.status.Phase != "ready" {
			writeAPIError(w, http.StatusConflict, errors.New("update is not ready to install"))
			return
		}
		if err := a.launch(a.status.Latest, a.staged, a.target); err != nil {
			a.status.Error = err.Error()
			writeJSON(w, http.StatusInternalServerError, a.status)
			return
		}
		a.status.Phase, a.status.Error = "restarting", ""
	default:
		writeAPIError(w, http.StatusBadRequest, errors.New("unknown update action"))
		return
	}
	writeJSON(w, http.StatusAccepted, a.status)
}

func (a *updateAPI) refresh() {
	result := a.check(a.ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.checked = time.Now()
	a.status.Phase = "idle"
	if result.Err != nil {
		return
	}
	a.status.Latest = result.LatestTag
	if result.Newer {
		a.status.Phase = "available"
	}
}

func (a *updateAPI) download(tag string) {
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Minute)
	defer cancel()
	staged, target, err := a.prepare(ctx, tag, updateProgress{api: a})
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.status.Phase, a.status.Error = "available", err.Error()
		return
	}
	if a.closed || a.ctx.Err() != nil {
		_ = os.Remove(staged)
		return
	}
	a.staged, a.target = staged, target
	a.status.Phase, a.status.Error = "ready", ""
}

type updateProgress struct{ api *updateAPI }

func (p updateProgress) Write(data []byte) (int, error) { return len(data), nil }

func (p updateProgress) DownloadProgress(downloaded, total int64) {
	p.api.mu.Lock()
	defer p.api.mu.Unlock()
	p.api.status.Downloaded, p.api.status.Total = downloaded, total
}
