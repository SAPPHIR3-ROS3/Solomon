package chat

import (
	"bytes"
	"strings"
	"testing"
)

func TestStreamPropagatesProviderFailures(t *testing.T) {
	for _, tc := range []struct{ event, detail string }{
		{`{"type":"error","error":{"message":"invalid tool history"}}`, "invalid tool history"},
		{`{"type":"response.failed","response":{"error":{"message":"model unavailable"}}}`, "model unavailable"},
		{`{"type":"response.incomplete","response":{"incomplete_details":{"reason":"max_output_tokens"}}}`, "max_output_tokens"},
	} {
		var out bytes.Buffer
		err := RewriteCodexSSEStream(strings.NewReader("data: "+tc.event+"\n\n"), &out, "test-model")
		if err == nil || !strings.Contains(err.Error(), tc.detail) {
			t.Fatalf("error=%v want %s", err, tc.detail)
		}
		if strings.Contains(out.String(), "[DONE]") {
			t.Fatalf("failure marked successful: %s", out.String())
		}
	}
}
