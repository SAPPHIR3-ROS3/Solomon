package server

import (
	"errors"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
	"net"
	"net/http"
)

func handleNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
	case http.MethodPatch:
		var request struct {
			Name    string `json:"name"`
			Enabled *bool  `json:"enabled"`
		}
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Enabled == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("enabled is required"))
			return
		}
		if err := networkpolicy.Set(request.Name, *request.Enabled); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
	default:
		w.Header().Set("Allow", "GET, PATCH")
		writeAPIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	rows, err := networkpolicy.List()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func allowNetworkRequest(w http.ResponseWriter, r *http.Request) bool {
	if local, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr); ok && !networkpolicy.AllowedLocal(local.IP) {
		http.Error(w, "Solomon access is disabled on this network interface", http.StatusForbidden)
		return false
	}
	p, err := networkpolicy.Read()
	if err != nil || !p.AllowedHostname(r.Host) {
		http.Error(w, "Solomon access via this MagicDNS name is disabled", http.StatusForbidden)
		return false
	}
	return true
}
