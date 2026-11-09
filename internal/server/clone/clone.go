package clone

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		json.NewEncoder(w).Encode(identities(r.URL.Query().Get("url")))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var request struct {
		URL         string `json:"url"`
		Destination string `json:"destination"`
		Identity    string `json:"identity"`
		Username    string `json:"username"`
		Password    string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&request) != nil {
		http.Error(w, "Invalid request", 400)
		return
	}
	request.URL = normalizeRemote(request.URL)
	remote, err := url.Parse(request.URL)
	if err != nil || (remote.Scheme != "https" && remote.Scheme != "ssh") || remote.Host == "" || (remote.User != nil && (remote.Scheme != "ssh" || hasPassword(remote))) {
		http.Error(w, "Use an HTTPS or SSH URL without embedded credentials", 400)
		return
	}
	home, _ := os.UserHomeDir()
	destination := request.Destination
	if strings.HasPrefix(destination, "~/") {
		destination = filepath.Join(home, destination[2:])
	}
	if !filepath.IsAbs(destination) {
		http.Error(w, "Enter an absolute destination folder", 400)
		return
	}
	existing := false
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() {
			http.Error(w, "Destination must be a directory", 400)
			return
		}
		entries, err := os.ReadDir(destination)
		if err != nil || len(entries) != 0 {
			http.Error(w, "Destination folder must be empty. Choose another folder or enter a subfolder name.", 400)
			return
		}
		existing = true
	} else if !os.IsNotExist(err) {
		http.Error(w, "Unable to inspect destination folder", 400)
		return
	}
	parent := filepath.Dir(destination)
	temporary, err := os.MkdirTemp(parent, ".solomon-clone-")
	if err != nil {
		http.Error(w, "Unable to create destination in parent folder", 400)
		return
	}
	defer os.RemoveAll(temporary)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	args := []string{}
	environment := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	if request.Identity == "anonymous" {
		args = append(args, "-c", "credential.helper=", "-c", "http.extraHeader=")
	} else if request.Identity == "credentials" {
		if remote.Scheme != "https" {
			http.Error(w, "Username and password/token require HTTPS", 400)
			return
		}
		args = append(args, "-c", "credential.helper=")
		environment = append(environment, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte(request.Username+":"+request.Password)))
	} else if request.Identity == "helper" || strings.HasPrefix(request.Identity, "helper:") || strings.HasPrefix(request.Identity, "gh:") {
		username, password := gitCredential(request.URL)
		if strings.HasPrefix(request.Identity, "gh:") {
			host, login, valid := strings.Cut(strings.TrimPrefix(request.Identity, "gh:"), ":")
			if !valid || host != remote.Hostname() {
				http.Error(w, "Identity does not match repository host", 400)
				return
			}
			username = login
			token, err := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", remote.Hostname(), "--user", username).Output()
			if err != nil {
				http.Error(w, "GitHub identity unavailable. Choose another identity.", 400)
				return
			}
			password = strings.TrimSpace(string(token))
		}
		if username == "" || password == "" {
			http.Error(w, "Credentials unavailable. Sign in again.", 400)
			return
		}
		args = append(args, "-c", "credential.helper=")
		environment = append(environment, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	} else if request.Identity != "anonymous" {
		found := false
		for _, identity := range identities(request.URL) {
			if identity.ID == request.Identity {
				found = true
			}
		}
		if !found {
			http.Error(w, "Identity unavailable", 400)
			return
		}
		if remote.Scheme == "https" {
			remote.Scheme = "ssh"
			remote.User = url.User("git")
			remote.Host = remote.Hostname()
			request.URL = remote.String()
		}
		key := filepath.Join(home, ".ssh", request.Identity)
		environment = append(environment, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o IdentitiesOnly=yes -i '"+strings.ReplaceAll(key, "'", "'\\''")+"'")
	}
	cloneTarget := temporary
	if existing {
		cloneTarget = destination
	}
	args = append(args, "clone", "--", request.URL, cloneTarget)
	command := exec.CommandContext(ctx, "git", args...)
	command.Env = environment
	started := time.Now()
	runErr := command.Run()
	logging.ProcessFinished(ctx, command, "repository.clone", started, runErr)
	if runErr != nil {
		http.Error(w, "Clone failed. Check repository URL, access rights and selected identity, or use other credentials.", 400)
		return
	}
	if !existing {
		if os.Rename(temporary, destination) != nil {
			http.Error(w, "Unable to move cloned repository to destination", 400)
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"path": destination, "displayPath": destination, "name": filepath.Base(destination)})
}
