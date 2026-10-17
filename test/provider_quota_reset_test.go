package test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

type resetQuotaTransport func(*http.Request) (*http.Response, error)

func (f resetQuotaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestChatGPTBankedResetRequestAndOutcomes(t *testing.T) {
	for _, code := range []string{"reset", "already_redeemed", "no_credit", "nothing_to_reset", "unexpected"} {
		t.Run(code, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: resetQuotaTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.String() != "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume" {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer fixture" || r.Header.Get("ChatGPT-Account-Id") != "account" {
					t.Fatal("missing account authentication")
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["redeem_request_id"] != "stable-id" || len(body) != 1 {
					t.Fatalf("unexpected payload: %#v", body)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":"` + code + `"}`)), Header: make(http.Header)}, nil
			})}
			got, err := server.ConsumeChatGPTResetForTest(context.Background(), "fixture", "account", "stable-id", client)
			if code == "unexpected" {
				if err == nil {
					t.Fatal("accepted unknown outcome")
				}
			} else if err != nil || got != code {
				t.Fatalf("got %q, %v", got, err)
			}
			if calls != 1 {
				t.Fatalf("request automatically retried %d times", calls)
			}
		})
	}
}

func TestChatGPTBankedResetAvailabilityIsStructured(t *testing.T) {
	for _, count := range []int{0, 2} {
		bars := server.QuotaBarsForTest(map[string]any{"available_count": float64(count)}, "chatgpt-resets")
		if len(bars) != 1 || bars[0].AvailableResets != count {
			t.Fatalf("unexpected reset availability: %#v", bars)
		}
	}
}
