package test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestCodexStreamFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name, payload string
		class         llm.ErrorClass
		status        int
	}{
		{"reported incident", `{"type":"error","error":{"type":"server_error","message":"An error occurred while processing your request. Request ID bc1aa83a-d5d1-49fe-80f1-86cbde3d56a6"}}`, llm.ErrorRetryable, 502},
		{"failed response", `{"type":"response.failed","response":{"error":{"code":"server_error","message":"Internal error"}}}`, llm.ErrorRetryable, 502},
		{"invalid request", `{"type":"error","error":{"type":"invalid_request_error","message":"Invalid history"}}`, llm.ErrorPermanent, 400},
		{"context limit", `{"type":"error","error":{"code":"context_length_exceeded","message":"Too long"}}`, llm.ErrorPermanent, 400},
		{"authentication", `{"type":"error","error":{"type":"authentication_error","message":"Expired"}}`, llm.ErrorPermanent, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := codexchat.RewriteCodexSSEStream(strings.NewReader("data: "+tc.payload+"\n\n"), &out, "test-model")
			class, status, _ := llm.ClassifyAPIError(err, false)
			if class != tc.class || status != tc.status {
				t.Fatalf("classification=(%v,%d), error=%v", class, status, err)
			}
			var upstream *codexchat.UpstreamError
			if !errors.As(err, &upstream) || len(upstream.Payload) == 0 {
				t.Fatalf("missing structured error: %v", err)
			}
			if !strings.Contains(string(upstream.Payload), `"message"`) {
				t.Fatalf("missing diagnostic payload: %s", upstream.Payload)
			}
		})
	}
}
