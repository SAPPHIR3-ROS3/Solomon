package test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/programming"
)

func TestProgrammingLanguagesPreservePreferencesAndStaleConfig(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	root := config.EmptyRoot()
	if err := config.Save(root); err != nil {
		t.Fatal(err)
	}
	stale, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	initial, err := config.UpdateProgrammingLanguage("python", config.ProgrammingLanguage{Name: "Python", Mode: "auto"}, true)
	if err != nil {
		t.Fatal(err)
	}
	no := false
	custom := initial
	custom.Mode = "custom"
	custom.Command = []string{"fixture-server", "--stdio"}
	custom.Diagnostics = &no
	custom.Settings = map[string]any{"python": map[string]any{"analysis": map[string]any{"typeCheckingMode": "strict"}}}
	if _, err = config.UpdateProgrammingLanguage("python", custom, false); err != nil {
		t.Fatal(err)
	}
	if _, err = config.UpdateProgrammingLanguage("python", config.ProgrammingLanguage{Name: "Python", Mode: "auto"}, true); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = config.Save(stale); err != nil {
			t.Fatal(err)
		}
	}
	got, err := config.ReadProgrammingLanguages()
	if err != nil {
		t.Fatal(err)
	}
	if got["python"].Mode != "custom" || !reflect.DeepEqual(got["python"].Command, custom.Command) || got["python"].Diagnostics == nil || *got["python"].Diagnostics || got["python"].FirstUsed != initial.FirstUsed {
		t.Fatalf("lost preferences: %+v", got)
	}
	if got["python"].Settings["python"] == nil {
		t.Fatal("lost server settings")
	}
	loaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProgrammingLanguages["python"].Mode != "custom" {
		t.Fatal("root loader dropped language section")
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("SOLOMON_HOME"), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("[programming_languages.python]")) {
		t.Fatalf("missing section: %s", data)
	}
}

func TestProgrammingLanguagesRejectInvalidConfiguration(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	for _, tc := range []struct {
		id    string
		value config.ProgrammingLanguage
	}{{"../python", config.ProgrammingLanguage{Name: "Python", Mode: "auto"}}, {"python", config.ProgrammingLanguage{Name: "Python", Mode: "custom"}}, {"python", config.ProgrammingLanguage{Name: "Python", Mode: "invalid"}}, {"python", config.ProgrammingLanguage{Name: "Python", Mode: "custom", Command: []string{"server\x00"}}}} {
		if _, err := config.UpdateProgrammingLanguage(tc.id, tc.value, false); err == nil {
			t.Fatalf("accepted invalid preference: %+v", tc)
		}
	}
}

func TestProgrammingLanguagesDiscoveryDoesNotTreatRuntimeAsServer(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	for _, id := range []string{"python", "go", "javascript", "unknown"} {
		if server := programming.Resolve(id, config.ProgrammingLanguage{Mode: "auto"}); server.Available {
			t.Fatalf("invented server: %+v", server)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	custom := programming.Resolve("unknown", config.ProgrammingLanguage{Mode: "custom", Command: []string{executable, "--fixture"}})
	if !custom.Available || len(custom.Command) != 2 || custom.Command[1] != "--fixture" {
		t.Fatalf("custom server unavailable: %+v", custom)
	}
	if server := programming.Resolve("unknown", config.ProgrammingLanguage{Mode: "fallback", Command: []string{executable}}); server.Available {
		t.Fatal("fallback launched custom executable")
	}
}

func TestProgrammingLanguageFrames(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"test","params":"€"}`)
	var stream bytes.Buffer
	for range 2 {
		if err := programming.WriteMessage(&stream, data); err != nil {
			t.Fatal(err)
		}
	}
	reader := bufio.NewReader(&stream)
	for range 2 {
		got, err := programming.ReadMessage(reader)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("invalid frame: %s, %v", got, err)
		}
	}
	for _, invalid := range []string{"Content-Length: -1\r\n\r\n", "Content-Length: 999999999\r\n\r\n", "X: y\r\n\r\n", strings.Repeat("X", 9000) + "\n"} {
		if _, err := programming.ReadMessage(bufio.NewReader(strings.NewReader(invalid))); err == nil {
			t.Fatal("accepted invalid frame")
		}
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
}

func TestProgrammingLanguageInstalledChoicesIncludeEveryDetectedServer(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PATH", directory)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
		t.Setenv("PATHEXT", ".EXE")
	}
	for _, name := range []string{"pyright-langserver", "pylsp"} {
		if err := os.WriteFile(filepath.Join(directory, name+suffix), []byte("fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	choices := programming.InstalledServers("python", config.ProgrammingLanguage{Mode: "fallback", Command: []string{"pyright-langserver", "--stdio"}})
	if len(choices) != 2 {
		t.Fatalf("expected both servers without duplicating custom selection: %+v", choices)
	}
	if filepath.Base(choices[0].Command[0]) != "pyright-langserver"+suffix || filepath.Base(choices[1].Command[0]) != "pylsp"+suffix {
		t.Fatalf("unexpected choices: %+v", choices)
	}
	if server := programming.Resolve("python", config.ProgrammingLanguage{Mode: "fallback"}); server.Available {
		t.Fatal("listing choices disabled fallback")
	}
}
