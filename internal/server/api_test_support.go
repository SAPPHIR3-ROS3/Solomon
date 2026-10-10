package server

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

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
