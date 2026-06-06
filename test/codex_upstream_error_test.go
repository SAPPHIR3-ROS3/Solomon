package test

import (
	"errors"
	"strings"
	"testing"

	codexchat "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/openai/codex/chat"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestUserFacingAPIError_chatGPTSubPrefixed(t *testing.T) {
	t.Parallel()
	err := errors.New(`ChatGPT Sub: model "gpt-5.4" is not available on your ChatGPT plan; use /models to pick another (free plan: gpt-5.4-mini)`)
	got := llm.UserFacingAPIError(err)
	for _, want := range []string{
		`model "gpt-5.4" is not available on your ChatGPT plan`,
		"/models",
		"gpt-5.4-mini",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Bad Request") || strings.Contains(got, "ChatGPT Sub:") || strings.Contains(got, "POST ") {
		t.Fatalf("unexpected raw HTTP text:\n%s", got)
	}
}

func TestUserFacingAPIError_codexDetailJSON(t *testing.T) {
	t.Parallel()
	raw := `POST "https://chatgpt.com/backend-api/codex/v1/chat/completions": 400 Bad Request {"detail":"something went wrong"}`
	got := llm.UserFacingAPIError(errors.New(raw))
	if !strings.Contains(got, "message: something went wrong") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestHumanizeCodexUpstreamError_unsupportedModel(t *testing.T) {
	t.Parallel()
	err := codexchat.ChatGPTSubUpstreamError(400, []byte(`{"detail":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."}`), "gpt-5.4")
	got := strings.TrimPrefix(err.Error(), "ChatGPT Sub: ")
	for _, want := range []string{`model "gpt-5.4"`, "/models", "gpt-5.4-mini"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestParseCodexUpstreamDetail(t *testing.T) {
	t.Parallel()
	err := codexchat.ChatGPTSubUpstreamError(400, []byte(`{"detail":"quota exceeded"}`), "")
	got := strings.TrimPrefix(err.Error(), "ChatGPT Sub: ")
	if got != "quota exceeded" {
		t.Fatalf("got %q", got)
	}
}

func TestChatGPTSubUpstreamError_wrapsMessage(t *testing.T) {
	t.Parallel()
	err := codexchat.ChatGPTSubUpstreamError(400, []byte(`{"detail":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."}`), "gpt-5.4")
	if !strings.HasPrefix(err.Error(), "ChatGPT Sub: ") {
		t.Fatalf("got %v", err)
	}
}
