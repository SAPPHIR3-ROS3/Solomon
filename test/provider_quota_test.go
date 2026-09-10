package test

import (
	"encoding/json"
	"math"
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
		{name: "Cursor summary", kind: "cursor", body: "{\"planUsage\":{\"includedSpend\":100,\"totalSpend\":25,\"autoPercentUsed\":20,\"apiPercentUsed\":12.5,\"totalPercentUsed\":25}}", want: []server.QuotaBarForTest{{Label: "Plan usage", Percent: 25}, {Label: "Auto models", Percent: 20}, {Label: "API models", Percent: 12.5}, {Label: "Total usage", Percent: 25}}},
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
			}
		})
	}
}
