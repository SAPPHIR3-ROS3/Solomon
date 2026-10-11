package test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestNotificationsAPIAndSlashOutput(t *testing.T) {
	logging.RecordNotification("system", "Notification API fixture")
	response := httptest.NewRecorder()
	server.NotificationsHandlerForTest()(response, httptest.NewRequest("GET", "/__solomon/notifications", nil))
	var entries []logging.Notification
	if err := json.Unmarshal(response.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Message == "Notification API fixture" {
			found = true
		}
	}
	if !found || response.Code != 200 {
		t.Fatal("notification missing from API")
	}
	for _, sample := range []struct{ output, want string }{
		{"===SYSTEM===\nSaved\n===SYSTEM===\n", ""},
		{"Command data\n===SYSTEM===\nSaved\n===SYSTEM===\nMore data", "Command data\n\nMore data"},
		{"\x1b[35m===SYSTEM===\x1b[0m\nNotice\n===SYSTEM===\n", ""},
	} {
		if got := server.SlashChatOutputForTest(sample.output); got != sample.want {
			t.Errorf("got %q, want %q", got, sample.want)
		}
	}
}
