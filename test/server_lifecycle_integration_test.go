package test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

// Runs the actual detached CLI, including its process supervisor. The fixture
// creates a grandchild so tree cleanup cannot pass by killing only Vite's PID.
func TestServerLifecycleIntegration(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("requires Node")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "solomon")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/solomon")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}

	variants := []string{"process-tree"}
	realEntry, _ := filepath.Abs("../gui/node_modules/vite/bin/vite.js")
	if _, err := os.Stat(realEntry); err == nil {
		variants = append(variants, "real-vite")
	}
	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("SOLOMON_HOME", filepath.Join(home, "home"))
			t.Setenv("SOLOMON_SERVER_PORT", strconv.Itoa(freeServerPortForTest(t)))
			gui := filepath.Join(home, "gui")
			entry := filepath.Join(gui, "node_modules", "vite", "bin", "vite.js")
			if err := os.MkdirAll(filepath.Dir(entry), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(gui, "src"), 0700); err != nil {
				t.Fatal(err)
			}
			write := func(path, data string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(gui, "package.json"), `{"type":"module"}`)
			write(filepath.Join(gui, "index.html"), `<main>lifecycle</main>`)
			childFile := filepath.Join(home, "child.pid")
			fixture := `const {spawn}=require('node:child_process'); const fs=require('node:fs');
const child=spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore'});
fs.writeFileSync(process.env.SOLOMON_LIFECYCLE_CHILD,String(child.pid));
require('node:http').createServer((req,res)=>res.end('lifecycle')).listen(Number(process.argv[process.argv.indexOf('--port')+1]),'127.0.0.1');`
			t.Setenv("SOLOMON_LIFECYCLE_CHILD", childFile)
			if variant == "real-vite" {
				entryPath := filepath.ToSlash(realEntry)
				if runtime.GOOS == "windows" {
					entryPath = "/" + entryPath
				}
				entryURL := (&url.URL{Scheme: "file", Path: entryPath}).String()
				fixture = "import(" + strconv.Quote(entryURL) + ");"
				// Windows temp paths can use 8.3 aliases while Vite canonicalizes its
				// root. This isolated fixture tests process cleanup, not the fs allowlist.
				write(filepath.Join(gui, "vite.config.mjs"), "export default {server:{fs:{strict:false}}};")
			}
			write(entry, fixture)
			// HOME is scoped to subprocesses, keeping Go's build/cache environment intact.
			env := append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
			cli := func(args ...string) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, binary, append([]string{"server"}, args...)...)
				command.Env, command.Dir = env, home
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("server %v: %v\n%s", args, err, output)
				}
			}
			owned := map[int]string{}
			load := func() serverruntime.State {
				t.Helper()
				state, err := serverruntime.LoadState()
				if err != nil {
					t.Fatal(err)
				}
				owned[state.PID], owned[state.VitePID] = processidentity.Read(state.PID), processidentity.Read(state.VitePID)
				if variant == "process-tree" {
					data, err := os.ReadFile(childFile)
					if err != nil {
						t.Fatal(err)
					}
					pid, err := strconv.Atoi(string(data))
					if err != nil {
						t.Fatal(err)
					}
					owned[pid] = processidentity.Read(pid)
				}
				return state
			}
			t.Cleanup(func() {
				if state, err := serverruntime.LoadState(); err == nil {
					owned[state.PID], owned[state.VitePID] = state.PIDIdentity, state.ViteIdentity
				}
				for pid, identity := range owned {
					if pid > 1 && identity != "" && processidentity.Read(pid) == identity {
						process, _ := os.FindProcess(pid)
						_ = process.Kill()
					}
				}
			})
			wait := func(label string, condition func() bool) {
				t.Helper()
				deadline := time.Now().Add(8 * time.Second)
				for time.Now().Before(deadline) {
					if condition() {
						return
					}
					time.Sleep(20 * time.Millisecond)
				}
				t.Fatal(label)
			}
			gone := func(label string, processes map[int]string) {
				t.Helper()
				wait(label, func() bool {
					for pid, identity := range processes {
						if identity != "" && processidentity.Read(pid) == identity {
							return false
						}
					}
					return true
				})
			}
			copyOwned := func() map[int]string {
				result := map[int]string{}
				for pid, identity := range owned {
					result[pid] = identity
				}
				return result
			}
			serves := func(address string) bool {
				response, err := (&http.Client{Timeout: 200 * time.Millisecond}).Get(address)
				if err != nil {
					return false
				}
				response.Body.Close()
				return response.StatusCode == 200
			}

			cli("start", "dev", gui)
			first := load()
			if !serves(first.ViteURL) {
				t.Fatal("Vite not serving")
			}
			previous := copyOwned()
			cli("restart")
			second := load()
			if first.PID == second.PID || first.VitePID == second.VitePID {
				t.Fatal("restart reused old processes")
			}
			gone("restart left old processes", previous)
			previous = copyOwned()
			cli("stop")
			gone("stop left processes", previous)

			if runtime.GOOS != "windows" {
				cli("start", "dev", gui)
				interrupted := load()
				process, _ := os.FindProcess(interrupted.PID)
				if err := process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
				gone("SIGTERM left daemon or descendants", copyOwned())
			}
			cli("start", "dev", gui)
			crashed := load()
			process, _ := os.FindProcess(crashed.PID)
			if err := process.Kill(); err != nil {
				t.Fatal(err)
			}
			previous = copyOwned()
			cli("start", "dev", gui)
			current := load()
			gone("daemon crash left Vite or descendants", previous)
			wait("Vite HTTP survived daemon crash", func() bool { return !serves(crashed.ViteURL) })
			if !serves(current.URL + "/health") {
				t.Fatal("daemon did not recover")
			}

			process, _ = os.FindProcess(current.VitePID)
			if err := process.Kill(); err != nil {
				t.Fatal(err)
			}
			wait("health still reports crashed Vite running", func() bool {
				health := getHealthForTest(t, current.URL)
				return health.Server.Vite == "stopped" && health.GUI == "unavailable"
			})
			// The daemon remains available even when the development frontend crashes.
			previous = copyOwned()
			delete(previous, current.PID)
			gone("Vite crash left descendants", previous)
			cli("stop")
			gone("stop after Vite crash left daemon", copyOwned())

			// Simulate reused PIDs with a real unrelated process and stale identities.
			sentinel := exec.Command("node", "-e", "setInterval(()=>{},1000)")
			if err := sentinel.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = sentinel.Process.Kill(); _ = sentinel.Wait() }()
			token := processidentity.Read(sentinel.Process.Pid)
			if token == "" {
				t.Fatal("cannot establish sentinel process identity")
			}
			stale := current
			stale.PID, stale.VitePID = sentinel.Process.Pid, sentinel.Process.Pid
			stale.PIDIdentity, stale.ViteIdentity = "stale", "stale"
			if err := serverruntime.SaveState(stale); err != nil {
				t.Fatal(err)
			}
			serverruntime.ForceStop(stale)
			serverruntime.ForceStop(serverruntime.State{VitePID: sentinel.Process.Pid, ViteIdentity: "stale"})
			cli("stop")
			cli("start", "normal")
			load()
			cli("stop")
			if processidentity.Read(sentinel.Process.Pid) != token {
				t.Fatal("unrelated PID was terminated")
			}
			// Also reject missing identities from legacy state files.
			stale.PIDIdentity, stale.ViteIdentity = "", ""
			serverruntime.ForceStop(stale)
			if processidentity.Read(sentinel.Process.Pid) != token {
				t.Fatal("legacy state terminated unrelated PID")
			}
			t.Logf("%s: restart, stop, daemon/Vite crash, descendants, recovery and stale PID passed", variant)
		})
	}
	if len(variants) == 1 {
		t.Log("real Vite dependencies unavailable; process-tree integration still executed")
	}
}
