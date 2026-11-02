package test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server/clone"
)

func TestCloneIntoExistingBaseFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	tools := t.TempDir()
	log := filepath.Join(tools, "arguments")
	t.Setenv("CLONE_ARGUMENT_LOG", log)
	if err := os.WriteFile(filepath.Join(tools, "git"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CLONE_ARGUMENT_LOG\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	destination := t.TempDir()
	send := func() *httptest.ResponseRecorder {
		payload, _ := json.Marshal(map[string]string{"url": "https://github.com/example/repo.git", "destination": destination, "identity": "anonymous"})
		response := httptest.NewRecorder()
		clone.Handle(response, httptest.NewRequest("POST", "/__solomon/git-clone", bytes.NewReader(payload)))
		return response
	}
	if response := send(); response.Code != 200 {
		t.Fatalf("empty base rejected: %s", response.Body.String())
	}
	arguments, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(arguments), destination+"\n") {
		t.Fatalf("clone did not target base: %s", arguments)
	}
	if err := os.WriteFile(filepath.Join(destination, "existing"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if response := send(); response.Code != 400 {
		t.Fatalf("nonempty base accepted: %d", response.Code)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "existing"))
	if err != nil || string(contents) != "preserve" {
		t.Fatal("existing contents changed")
	}
}
