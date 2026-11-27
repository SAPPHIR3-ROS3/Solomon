package server

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
)

func HandleDeviceLinks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	host := requestHostname(r.Host)
	if !isLoopbackHost(remote) || (!strings.EqualFold(host, "localhost") && !isLoopbackHost(host)) || !terminalOriginAllowed(r) {
		writeAPIError(w, http.StatusForbidden, errors.New("device links are available only from the local host"))
		return
	}
	if r.Method == http.MethodPatch {
		var request struct {
			Interface string `json:"interface"`
			IP        string `json:"ip"`
			Allowed   *bool  `json:"allowed"`
		}
		if err := decodeJSONBody(w, r, &request, 4096); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if request.Allowed == nil {
			writeAPIError(w, http.StatusBadRequest, errors.New("allowed is required"))
			return
		}
		if err := networkpolicy.SetDevice(request.Interface, request.IP, *request.Allowed); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	links, err := networkpolicy.ListLinks(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	policy, err := networkpolicy.Read()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	for _, link := range links {
		for i := range link.Devices {
			link.Devices[i].Allowed = policy.AllowedDevices[link.Name][link.Devices[i].IP]
		}
	}
	writeJSON(w, http.StatusOK, links)
}

func allowDeviceRequest(w http.ResponseWriter, r *http.Request) bool {
	local, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr)
	if !ok {
		return true
	}
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	if local.IP.IsLoopback() && isLoopbackHost(remote) {
		return true
	}
	policy, err := networkpolicy.Read()
	if err == nil && policy.AllowsDevice(local.IP, net.ParseIP(remote)) {
		return true
	}
	http.Error(w, "This device is not allowed to connect to Solomon", http.StatusForbidden)
	return false
}
