package server

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

type DevelopmentFrontendForTest struct{ frontend *developmentFrontend }

func NewDevelopmentFrontendForTest(state State, handler http.Handler) *DevelopmentFrontendForTest {
	return &DevelopmentFrontendForTest{frontend: &developmentFrontend{state: state, handler: handler}}
}

func (d *DevelopmentFrontendForTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.frontend.ServeHTTP(w, r)
}
func (d *DevelopmentFrontendForTest) Change(ctx context.Context, mode, directory string) (State, error) {
	return d.frontend.change(ctx, mode, directory)
}
func (d *DevelopmentFrontendForTest) Snapshot() State                { return d.frontend.snapshot() }
func (d *DevelopmentFrontendForTest) Close()                         { d.frontend.close() }
func ExportGUISourcesForTest() (string, error)                       { return exportGUISources() }
func StartViteForTest(directory string) (*url.URL, *exec.Cmd, error) { return startVite(directory) }
func StopViteForTest(command *exec.Cmd)                              { stopManagedProcess(command) }

// ViteCommandForTest exposes the dev launcher without starting the daemon.
func ViteCommandForTest(directory string, port int) *exec.Cmd {
	return viteCommand(directory, port)
}

// GlobalAgentsHandlerForTest exposes the handler to tests in test/.
func GlobalAgentsHandlerForTest() http.HandlerFunc {
	return newCustomizationAPI().handleGlobalAgents
}

// UpdateAPIForTest provides controlled dependencies and synchronized state access
// for the external update API tests.
type UpdateAPIForTest struct{ api *updateAPI }

func NewUpdateAPIForTest(ctx context.Context, build State) *UpdateAPIForTest {
	return &UpdateAPIForTest{api: newUpdateAPI(ctx, build)}
}

func (a *UpdateAPIForTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.api.handle(w, r)
}

func (a *UpdateAPIForTest) SetCheck(check func(context.Context) updater.CheckResult) {
	a.api.check = check
}

func (a *UpdateAPIForTest) SetPrepare(prepare func(context.Context, string, io.Writer) (string, string, error)) {
	a.api.prepare = prepare
}

func (a *UpdateAPIForTest) SetLaunch(launch func(string, string, string) error) {
	a.api.launch = launch
}

func (a *UpdateAPIForTest) Status() (phase, message string) {
	a.api.mu.Lock()
	defer a.api.mu.Unlock()
	return a.api.status.Phase, a.api.status.Error
}

func (a *UpdateAPIForTest) SetStatus(phase, latest, staged string) {
	a.api.mu.Lock()
	defer a.api.mu.Unlock()
	a.api.status.Phase, a.api.status.Latest = phase, latest
	a.api.staged = staged
	a.api.checked = time.Now()
}

// TailscaleMagicDNSAddressForTest exposes status parsing to tests in test/.
func TailscaleMagicDNSAddressForTest(addresses []ReachableAddress, port int, data []byte) (ReachableAddress, bool) {
	return tailscaleMagicDNSAddress(addresses, port, data)
}

func NotificationsHandlerForTest() http.HandlerFunc { return handleNotifications }

func SlashChatOutputForTest(output string) string { return slashChatOutput(output) }

// BackgroundAPIForTest allows power-management tests to use fake inhibitors.
type BackgroundAPIForTest struct{ api *backgroundAPI }

func NewBackgroundAPIForTest(acquire func() (func(), error), save func(bool) error) *BackgroundAPIForTest {
	return &BackgroundAPIForTest{api: &backgroundAPI{acquire: acquire, save: save}}
}

func (a *BackgroundAPIForTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.api.handle(w, r)
}

func (a *BackgroundAPIForTest) Close() { a.api.close() }

func (a *BackgroundAPIForTest) Enabled() bool {
	a.api.mu.Lock()
	defer a.api.mu.Unlock()
	return a.api.release != nil
}
