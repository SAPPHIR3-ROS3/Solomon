package codex

import (
	"strings"
	"testing"
)

func TestHumanizeCodexUpstreamError_unsupportedModel(t *testing.T) {
	t.Parallel()
	detail := `The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account.`
	got := humanizeCodexUpstreamError(400, detail, "gpt-5.4")
	for _, want := range []string{`model "gpt-5.4"`, "/models", "gpt-5.4-mini"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestParseCodexUpstreamDetail(t *testing.T) {
	t.Parallel()
	body := []byte(`{"detail":"quota exceeded"}`)
	if got := parseCodexUpstreamDetail(body); got != "quota exceeded" {
		t.Fatalf("got %q", got)
	}
}

func TestChatGPTSubUpstreamError_wrapsMessage(t *testing.T) {
	t.Parallel()
	err := chatGPTSubUpstreamError(400, []byte(`{"detail":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."}`), "gpt-5.4")
	if !strings.HasPrefix(err.Error(), "ChatGPT Sub: ") {
		t.Fatalf("got %v", err)
	}
}
