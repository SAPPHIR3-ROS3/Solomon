//go:build linux || darwin

package test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

// Real CLI, PTY, REPL commands, checksum verification, installation and detached
// coordinator. An overlay redirects release HTTP to a local fixture and seeds
// the OLD runtime's chat, allowing startup autoupdate to run with an active chat.
// The replacement runtime uses its normal session startup path.
func TestTUIUpgradeReturnsToSameTerminalAndChat(t *testing.T) {
	const oldTag, newTag = "v2099.1.0", "v2099.1.1"
	root := t.TempDir()
	repo, _ := filepath.Abs("..")
	hook := `package main
import("net/http";"net/url";"os";"encoding/json";"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater";"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands")
type fixtureTransport struct { base *url.URL; next http.RoundTripper }
func(t fixtureTransport) RoundTrip(r *http.Request)(*http.Response,error){
 if r.URL.Host != "github.com" && r.URL.Host != "api.github.com" { return t.next.RoundTrip(r) }; c:=r.Clone(r.Context()); u:=*r.URL; c.URL=&u
 c.URL.Scheme=t.base.Scheme; c.URL.Host=t.base.Host; c.Host=t.base.Host
 return t.next.RoundTrip(c)
}
func init(){ if address:=os.Getenv("SOLOMON_UPGRADE_FIXTURE"); address!="" { base,_:=url.Parse(address); http.DefaultTransport=fixtureTransport{base,http.DefaultTransport}; updater.SetLatestReleaseAPIURL(address+"/latest"); cli:=os.Getenv("SOLOMON_BINARY"); desktop,_:=updater.DesktopExecutablePath(cli); marker,_:=json.Marshal(map[string]string{"tag":commands.LocalVersionString(),"cli":cli}); os.WriteFile(desktop+".install.json",marker,0600) } }
`
	hookPath := filepath.Join(root, "hook.go")
	if err := os.WriteFile(hookPath, []byte(hook), 0600); err != nil {
		t.Fatal(err)
	}
	mainSource, err := os.ReadFile(filepath.Join(repo, "cmd/solomon/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	seeded := strings.Replace(string(mainSource), "multiline.EnsureCookedTTY()\n\trt := agentruntime.NewRuntime", `if commands.LocalVersionString() == "`+oldTag+`" {
 if id := os.Getenv("SOLOMON_UPGRADE_SEED_CHAT"); id != "" { loaded, err := chatstore.ReadSession(hex,id); if err != nil { panic(err) }; sess = loaded }
 }
 multiline.EnsureCookedTTY()
 rt := agentruntime.NewRuntime`, 1)
	if seeded == string(mainSource) {
		t.Fatal("fixture startup insertion point missing")
	}
	mainPath := filepath.Join(root, "main.go")
	if err := os.WriteFile(mainPath, []byte(seeded), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(repo, "cmd/solomon/main.go"): mainPath, filepath.Join(repo, "cmd/solomon/zz_upgrade_fixture.go"): hookPath}})
	overlayPath := filepath.Join(root, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	build := func(tag, path string) {
		t.Helper()
		cmd := exec.Command("go", "build", "-buildvcs=false", "-overlay", overlayPath, "-ldflags=-X github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands.version="+tag, "-o", path, "./cmd/solomon")
		cmd.Dir = repo
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, output)
		}
	}
	oldBinary, newBinary := filepath.Join(root, "old"), filepath.Join(root, "new")
	build(oldTag, oldBinary)
	build(newTag, newBinary)
	newBytes, err := os.ReadFile(newBinary)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := updater.ReleaseAssetName(newTag)
	if err != nil {
		t.Fatal(err)
	}
	checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(newBytes), asset)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/latest":
			fmt.Fprintf(w, `{"tag_name":%q}`, newTag)
		case strings.HasSuffix(r.URL.Path, "/checksums.txt"):
			io.WriteString(w, checksum)
		case strings.HasSuffix(r.URL.Path, "/"+asset):
			w.Write(newBytes)
		case strings.HasSuffix(r.URL.Path, "/models"):
			io.WriteString(w, `{"data":[{"id":"fixture"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fixture.Close()
	for _, mode := range []string{"upgrade", "autoupdate"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			appHome := filepath.Join(home, "home")
			workspace := filepath.Join(home, "workspace")
			if err := os.MkdirAll(appHome, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(workspace, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("SOLOMON_HOME", appHome)
			t.Setenv("SOLOMON_SERVER_PORT", fmt.Sprint(freeServerPortForTest(t)))
			binary := filepath.Join(home, "solomon")
			oldBytes, _ := os.ReadFile(oldBinary)
			if err := os.WriteFile(binary, oldBytes, 0700); err != nil {
				t.Fatal(err)
			}
			// Provision a desktop marker so this TUI test never downloads or launches a GUI.
			desktop, err := updater.DesktopExecutablePath(binary)
			if err != nil {
				t.Fatal(err)
			}
			os.MkdirAll(filepath.Dir(desktop), 0700)
			os.WriteFile(desktop, []byte("fixture"), 0700)
			for _, tag := range []string{oldTag} {
				marker, _ := json.Marshal(map[string]string{"tag": tag, "cli": binary})
				os.WriteFile(desktop+".install.json", marker, 0600)
			}
			config := fmt.Sprintf("user_name='Fixture'\nautoupdate=%t\n[current]\nprovider='fixture'\nmodel='fixture'\n[providers.fixture]\nbase_url='%s'\napi_key='fixture-not-a-secret'\n", mode == "autoupdate", fixture.URL)
			if err := os.WriteFile(filepath.Join(appHome, "config.toml"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			_, hex, err := project.Resolve(workspace)
			if err != nil {
				t.Fatal(err)
			}
			chat := &chatstore.Session{ID: "upgrade-session", Title: "Upgrade session", CreatedAt: time.Now(), LastMessageAt: time.Now(), Messages: []chatstore.Message{{Role: "user", Content: "preserve-this-exact-chat"}}}
			if err := chatstore.WriteSession(hex, chat); err != nil {
				t.Fatal(err)
			}
			env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "SOLOMON_BINARY="+binary, "SOLOMON_UPGRADE_FIXTURE="+fixture.URL, "SOLOMON_UPGRADE_SEED_CHAT="+chat.ID, "TERM=xterm-256color")
			command := exec.Command(binary, workspace)
			command.Env, command.Dir = env, workspace
			terminal, err := pty.StartWithSize(command, &pty.Winsize{Rows: 40, Cols: 140})
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			var mu sync.Mutex
			var transcript strings.Builder
			go func() {
				buffer := make([]byte, 32768)
				for {
					n, err := terminal.Read(buffer)
					if n > 0 {
						mu.Lock()
						transcript.Write(buffer[:n])
						mu.Unlock()
					}
					if err != nil {
						return
					}
				}
			}()
			output := func() string { mu.Lock(); defer mu.Unlock(); return ansi.Strip(transcript.String()) }
			exited := make(chan error, 1)
			go func() { exited <- command.Wait() }()
			t.Cleanup(func() {
				if state, err := serverruntime.LoadState(); err == nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					request, _ := http.NewRequestWithContext(ctx, http.MethodPost, state.URL+"/_solomon/stop", nil)
					if response, err := http.DefaultClient.Do(request); err == nil {
						response.Body.Close()
					}
				}
				_ = command.Process.Kill()
				select {
				case <-exited:
				case <-time.After(5 * time.Second):
				}
			})
			wait := func(label string, condition func() bool) bool {
				t.Helper()
				deadline := time.Now().Add(35 * time.Second)
				for time.Now().Before(deadline) {
					if condition() {
						return true
					}
					time.Sleep(50 * time.Millisecond)
				}
				log, _ := os.ReadFile(filepath.Join(appHome, "logs/update/update.log"))
				t.Errorf("%s\nterminal:\n%s\nupdate:\n%s", label, output(), log)
				return false
			}
			outerIdentity := processidentity.Read(command.Process.Pid)
			if outerIdentity == "" {
				t.Fatal("cannot establish TUI process identity")
			}
			outerTTY, _ := os.Readlink(fmt.Sprintf("/proc/%d/fd/0", command.Process.Pid))
			if mode == "upgrade" {
				if !wait("initial REPL never ready", func() bool { return strings.Contains(output(), oldTag) && strings.Contains(output(), "You:") }) {
					return
				}
				for _, key := range []byte("/upgrade\r") {
					if _, err := terminal.Write([]byte{key}); err != nil {
						t.Fatal(err)
					}
					time.Sleep(30 * time.Millisecond)
				}
			}
			if !wait("new TUI never reopened", func() bool {
				return func() bool {
					out := output()
					index := strings.LastIndex(out, "Welcome to Solomon")
					return strings.Count(out, "Welcome to Solomon") >= 2 && index >= 0 && strings.Contains(out[index:], newTag) && strings.Contains(out[index:], "You:")
				}() && func() bool { s, e := serverruntime.LoadState(); return e == nil && s.Version == newTag }()
			}) {
				return
			}
			if processidentity.Read(command.Process.Pid) != outerIdentity {
				t.Error("outer terminal client exited instead of re-entering in place")
			}
			if runtimeTTY, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/0", command.Process.Pid)); err == nil && runtimeTTY != outerTTY {
				t.Error("outer terminal changed")
			}
			// Export current, not last/by-ID: this proves the active chat was restored.
			for _, key := range []byte("/export current\r") {
				terminal.Write([]byte{key})
				time.Sleep(30 * time.Millisecond)
			}
			if !wait("active chat was not restored", func() bool {
				found := false
				filepath.WalkDir(filepath.Join(appHome, "exported"), func(path string, entry os.DirEntry, err error) error {
					if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".md") {
						data, _ := os.ReadFile(path)
						if strings.Contains(string(data), "preserve-this-exact-chat") {
							found = true
						}
					}
					return nil
				})
				return found
			}) {
				return
			}
			records, err := filepath.Glob(filepath.Join(appHome, "run", "terminal-chats", "*.json"))
			if err != nil || len(records) != 1 {
				t.Fatalf("terminal ID changed across upgrade: %v (%v)", records, err)
			}
			terminalID := strings.TrimSuffix(filepath.Base(records[0]), ".json")
			restored, err := chatstore.ReadTerminalChat(terminalID, hex)
			if err != nil || restored.ID != chat.ID {
				t.Fatalf("wrong active chat: %+v (%v)", restored, err)
			}
			t.Log("actual upgrade returned to the same terminal and active chat")
		})
	}
}
