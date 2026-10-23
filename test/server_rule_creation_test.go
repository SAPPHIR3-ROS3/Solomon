package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/instructions"
	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestServerRuntime_CreateRule(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"text":"   "}`, http.StatusBadRequest},
		{`{"text":" First rule "}`, http.StatusOK},
		{`{"text":"Second rule"}`, http.StatusOK},
	} {
		response, err := http.Post(server.URL+"/__solomon/rules", "application/json", bytes.NewBufferString(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != tc.status {
			t.Fatalf("status=%d want=%d", response.StatusCode, tc.status)
		}
	}
	response, err := http.Get(server.URL + "/__solomon/rules")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Rules []struct {
			Enabled bool   `json:"enabled"`
			ID      int    `json:"id"`
			Text    string `json:"text"`
		} `json:"rules"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rules) != 2 || payload.Rules[0].Text != "First rule" || payload.Rules[1].Text != "Second rule" || payload.Rules[1].ID != 2 || !payload.Rules[0].Enabled {
		t.Fatalf("unexpected rules: %+v", payload.Rules)
	}
	if _, err := instructions.SetRuleEnabled(instructions.ScopeGlobal, "", 1, false); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = http.Get(server.URL + "/__solomon/rules")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if len(payload.Rules) != 2 || payload.Rules[0].Enabled {
		t.Fatalf("inactive status missing: %+v", payload.Rules)
	}
	response, err = http.Post(server.URL+"/__solomon/rules/reorder", "application/json", bytes.NewBufferString("{\"ruleIds\":[2,1]}"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		response.Body.Close()
		t.Fatal(err)
	}
	response.Body.Close()
	if len(payload.Rules) != 2 || payload.Rules[0].Text != "Second rule" || !payload.Rules[0].Enabled || payload.Rules[1].Text != "First rule" || payload.Rules[1].Enabled {
		t.Fatalf("reorder changed activation state: %+v", payload.Rules)
	}
}
