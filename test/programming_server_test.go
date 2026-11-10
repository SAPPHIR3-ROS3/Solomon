package test

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/programming"
	"github.com/gorilla/websocket"
)

func TestProgrammingLanguageProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "solomon-lsp-fixture" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		data, err := programming.ReadMessage(reader)
		if err != nil {
			os.Exit(0)
		}
		var message map[string]any
		if json.Unmarshal(data, &message) != nil {
			os.Exit(2)
		}
		if message["method"] == "exit" {
			os.Exit(0)
		}
		var response map[string]any
		if id, ok := message["id"]; ok {
			response = map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"capabilities": map[string]any{"textDocumentSync": 1}}}
		}
		if message["method"] == "textDocument/didOpen" {
			cwd, _ := os.Getwd()
			response = map[string]any{"jsonrpc": "2.0", "method": "fixture/opened", "params": map[string]any{"cwd": cwd, "document": message["params"]}}
		}
		if response != nil {
			payload, _ := json.Marshal(response)
			if programming.WriteMessage(os.Stdout, payload) != nil {
				os.Exit(3)
			}
		}
	}
}

func TestProgrammingLanguageWebSocketUsesProjectAndConfiguredProcess(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.fixture"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint := projectFileOperationEndpoint(t, workspace)
	base := strings.Split(endpoint, "/__solomon/")[0]
	project := strings.Repeat("a", 64)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	language := config.ProgrammingLanguage{Name: "Fixture", Mode: "custom", Command: []string{executable, "-test.run=^TestProgrammingLanguageProcess$", "--", "solomon-lsp-fixture"}}
	if _, err = config.UpdateProgrammingLanguage("fixture", language, true); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project": {project}, "path": {"main.fixture"}, "language": {"fixture"}}
	target := strings.Replace(base, "http:", "ws:", 1) + "/__solomon/language-server?" + query.Encode()
	connection, response, err := websocket.DefaultDialer.Dial(target, http.Header{"Origin": []string{"https://untrusted.example"}})
	if connection != nil {
		connection.Close()
	}
	if err == nil || response == nil || response.StatusCode != 403 {
		t.Fatalf("untrusted origin accepted: %v, %v", response, err)
	}
	if response != nil {
		response.Body.Close()
	}
	connection, _, err = websocket.DefaultDialer.Dial(target, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err = connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err = connection.ReadJSON(&result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != float64(1) || result["result"] == nil {
		t.Fatalf("invalid initialize response: %v", result)
	}
	if err = connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"text": "fixture"}}); err != nil {
		t.Fatal(err)
	}
	if err = connection.ReadJSON(&result); err != nil {
		t.Fatal(err)
	}
	cwd := result["params"].(map[string]any)["cwd"].(string)
	want, _ := filepath.EvalSymlinks(workspace)
	if cwd != workspace && cwd != want {
		t.Fatalf("server used wrong workspace: %q", cwd)
	}
	query.Set("path", "../outside.fixture")
	invalid := strings.Replace(base, "http:", "ws:", 1) + "/__solomon/language-server?" + query.Encode()
	other, response, err := websocket.DefaultDialer.Dial(invalid, nil)
	if other != nil {
		other.Close()
	}
	if err == nil || response == nil || response.StatusCode != 400 {
		t.Fatalf("escaped workspace: %v, %v", response, err)
	}
	if response != nil {
		response.Body.Close()
	}
	documentResponse, err := http.Get(base + "/__solomon/language-document?" + url.Values{"project": {project}, "path": {"main.fixture"}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer documentResponse.Body.Close()
	var document map[string]string
	if err = json.NewDecoder(documentResponse.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(document["uri"], "file:///") || !strings.HasSuffix(document["uri"], "main.fixture") {
		t.Fatalf("invalid document URI: %v", document)
	}
	settingsResponse, err := http.Get(base + "/__solomon/programming-languages")
	if err != nil {
		t.Fatal(err)
	}
	defer settingsResponse.Body.Close()
	data, _ := io.ReadAll(settingsResponse.Body)
	if !strings.Contains(string(data), `"fixture"`) {
		t.Fatalf("missing persisted settings: %s", data)
	}
}

func TestProgrammingLanguageInstalledDartSDK(t *testing.T) {
	dart, err := exec.LookPath("dart")
	if err != nil {
		t.Skip("Dart SDK is not installed")
	}
	bundled := filepath.Join(filepath.Dir(dart), "cache", "dart-sdk", "bin", "dart")
	if info, err := os.Stat(bundled); err == nil && !info.IsDir() {
		dart = bundled
	}
	if detected := programming.Resolve("dart", config.ProgrammingLanguage{Mode: "auto"}); !detected.Available || detected.Command[0] != dart {
		t.Fatalf("Dart SDK was not detected directly: %+v", detected)
	}
	workspace := t.TempDir()
	source := "void main() { int answer = 'text'; }\n"
	if err = os.WriteFile(filepath.Join(workspace, "main.dart"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	endpoint := projectFileOperationEndpoint(t, workspace)
	base := strings.Split(endpoint, "/__solomon/")[0]
	project := strings.Repeat("a", 64)
	if _, err = config.UpdateProgrammingLanguage("dart", config.ProgrammingLanguage{Name: "Dart", Mode: "auto"}, true); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project": {project}, "path": {"main.dart"}, "language": {"dart"}}
	response, err := http.Get(base + "/__solomon/language-document?" + query.Encode())
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]string
	if err = json.NewDecoder(response.Body).Decode(&document); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	connection, _, err := websocket.DefaultDialer.Dial(strings.Replace(base, "http:", "ws:", 1)+"/__solomon/language-server?"+query.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(30 * time.Second))
	if err = connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"processId": nil, "rootUri": document["rootUri"], "capabilities": map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	for {
		var message map[string]any
		if err = connection.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message["id"] == float64(1) && message["method"] == nil {
			if message["error"] != nil {
				t.Fatalf("Dart initialize error: %v", message)
			}
			break
		}
		if message["id"] != nil && message["method"] != nil {
			connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": message["id"], "result": nil})
		}
	}
	if err = connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err = connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": document["uri"], "languageId": "dart", "version": 1, "text": source}}}); err != nil {
		t.Fatal(err)
	}
	for {
		var message map[string]any
		if err = connection.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		if message["method"] == "textDocument/publishDiagnostics" {
			params := message["params"].(map[string]any)
			if params["uri"] == document["uri"] && len(params["diagnostics"].([]any)) > 0 {
				return
			}
		}
		if message["id"] != nil && message["method"] != nil {
			connection.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": message["id"], "result": nil})
		}
	}
}
