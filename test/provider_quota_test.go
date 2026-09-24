package test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	server "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestProviderQuotaParsers(t *testing.T) {
	tests := []struct {
		name string
		kind string
		body string
		want []server.QuotaBarForTest
	}{
		{name: "ChatGPT wham windows", kind: "chatgpt", body: "{\"rate_limit\":{\"primary_window\":{\"used_percent\":9,\"limit_window_seconds\":18000},\"secondary_window\":{\"used_percent\":33,\"limit_window_seconds\":604800}},\"spend_control\":{\"individual_limit\":{\"used_percent\":4}}}", want: []server.QuotaBarForTest{{Label: "5-hour limit", Percent: 9}, {Label: "Weekly limit", Percent: 33}, {Label: "Spend control", Percent: 4}}},
		{name: "Claude utilization", kind: "claude", body: "{\"five_hour\":{\"utilization\":0.15},\"seven_day\":{\"utilization\":42}}", want: []server.QuotaBarForTest{{Label: "5-hour limit", Percent: 15}, {Label: "Weekly limit", Percent: 42}}},
		{name: "Cursor dashboard buckets", kind: "cursor", body: "{\"planUsage\":{\"includedSpend\":100,\"totalSpend\":25,\"autoPercentUsed\":7,\"apiPercentUsed\":11,\"totalPercentUsed\":7}}", want: []server.QuotaBarForTest{{Label: "Cursor Models", Percent: 7, Detail: "Includes Cursor Grok and Composer"}, {Label: "Other Models", Percent: 11}}},
		{name: "Cursor empty plan still shows bars", kind: "cursor", body: "{\"planUsage\":{}}", want: []server.QuotaBarForTest{{Label: "Cursor Models", Detail: "Includes Cursor Grok and Composer"}, {Label: "Other Models"}}},
		{name: "ChatGPT banked resets", kind: "chatgpt-resets", body: "{\"available_count\":3,\"credits\":[{\"status\":\"available\",\"expires_at\":\"2099-01-15T12:00:00Z\"},{\"status\":\"available\",\"expires_at\":\"2099-06-01T12:00:00Z\"},{\"status\":\"used\",\"expires_at\":\"2099-01-01T12:00:00Z\"}]}", want: []server.QuotaBarForTest{{Label: "Banked resets", Detail: "3 available · nearest expires"}}},
		{name: "ChatGPT usage with banked count", kind: "chatgpt", body: "{\"rate_limit\":{\"primary_window\":{\"used_percent\":9,\"limit_window_seconds\":18000},\"secondary_window\":{\"used_percent\":33,\"limit_window_seconds\":604800}},\"rate_limit_reset_credits\":{\"available_count\":2}}", want: []server.QuotaBarForTest{{Label: "5-hour limit", Percent: 9}, {Label: "Weekly limit", Percent: 33}, {Label: "Banked resets", Detail: "2 available"}}},
		{name: "OpenRouter credits", kind: "openrouter", body: "{\"data\":{\"total_credits\":10,\"total_usage\":2.5}}", want: []server.QuotaBarForTest{{Label: "Credits usage", Percent: 25}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload any
			if err := json.Unmarshal([]byte(tc.body), &payload); err != nil {
				t.Fatal(err)
			}
			got := server.QuotaBarsForTest(payload, tc.kind)
			if len(got) != len(tc.want) {
				t.Fatalf("bars=%v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i].Label != tc.want[i].Label || math.Abs(got[i].Percent-tc.want[i].Percent) > 0.001 {
					t.Errorf("bar[%d]=%+v, want %+v", i, got[i], tc.want[i])
				}
				if tc.want[i].Detail != "" && !strings.Contains(got[i].Detail, tc.want[i].Detail) {
					t.Errorf("bar[%d].Detail=%q, want to contain %q", i, got[i].Detail, tc.want[i].Detail)
				}
			}
		})
	}
}

func TestQuotaErrorFromBody(t *testing.T) {
	tests := []struct {
		name   string
		status string
		body   string
		want   string
	}{
		{name: "json error string", status: "401 Unauthorized", body: "{\"error\":\"failed to load user profile\"}", want: "failed to load user profile"},
		{name: "nested message", status: "401 Unauthorized", body: "{\"error\":{\"message\":\"failed to load user profile\"}}", want: "failed to load user profile"},
		{name: "plain text", status: "401 Unauthorized", body: "failed to load user profile", want: "failed to load user profile"},
		{name: "html fallback", status: "502 Bad Gateway", body: "<html>nope</html>", want: "502 Bad Gateway"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := server.QuotaErrorFromBodyForTest(tc.status, []byte(tc.body))
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
