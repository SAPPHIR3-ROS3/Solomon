package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	guibundle "github.com/SAPPHIR3-ROS3/Solomon/v2026/gui"
)

func TestDesktopFrontendSurvivesUnavailableDaemon(t *testing.T) {
	if !guibundle.Ready() {
		t.Skip("frontend has not been bundled")
	}
	t.Setenv("SOLOMON_HOME", t.TempDir())
	for _, staleState := range []bool{false, true} {
		if staleState {
			closed := httptest.NewServer(nil)
			address := closed.URL
			closed.Close()
			if err := server.SaveState(server.State{PID: 12345, URL: address, StartedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}
		response := httptest.NewRecorder()
		server.DesktopFrontend().ServeHTTP(response, httptest.NewRequest("GET", "/settings", nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "<html") {
			t.Fatalf("offline settings unavailable: %d %s", response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest("GET", "/settings", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()
	server.DesktopFrontend().ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("offline fallback must keep origin validation")
	}
}
