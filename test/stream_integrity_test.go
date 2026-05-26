package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/llm"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func streamIDMismatchSSEBody() string {
	c1 := `{"id":"chatcmpl-aaa","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"reasoning_content":"pre-reject","content":"visible"},"finish_reason":null}]}`
	c2 := `{"id":"chatcmpl-bbb","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"content":"must-not-appear"},"finish_reason":null}]}`
	return "data: " + c1 + "\n\ndata: " + c2 + "\n\ndata: [DONE]\n\n"
}

func mockStreamClient(t *testing.T, body string) openai.Client {
	t.Helper()
	return openai.NewClient(
		option.WithBaseURL("http://127.0.0.1:9"),
		option.WithAPIKey("test"),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			_ = req
			_ = next
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	)
}

func TestStreamTextRejectsIncoherentStreamID(t *testing.T) {
	client := mockStreamClient(t, streamIDMismatchSSEBody())
	var out bytes.Buffer
	_, _, err := llm.StreamText(context.Background(), client, openai.ChatCompletionNewParams{
		Model: "test",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hi"),
		},
	}, &out, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error for incoherent stream")
	}
	if !errors.Is(err, llm.ErrStreamAccumulatorRejected) {
		t.Fatalf("expected ErrStreamAccumulatorRejected, got %v", err)
	}
	if strings.Contains(out.String(), "must-not-appear") {
		t.Fatalf("forged chunk content must not be emitted, got %q", out.String())
	}
}

func TestStreamAssistantTurnRejectsIncoherentStreamID(t *testing.T) {
	client := mockStreamClient(t, streamIDMismatchSSEBody())
	turn, err := llm.StreamAssistantTurn(context.Background(), client, openai.ChatCompletionNewParams{
		Model: "test",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hi"),
		},
	}, io.Discard, llm.StreamOpts{})
	if err == nil {
		t.Fatal("expected error for incoherent stream")
	}
	if !errors.Is(err, llm.ErrStreamAccumulatorRejected) {
		t.Fatalf("expected ErrStreamAccumulatorRejected, got %v", err)
	}
	if strings.TrimSpace(turn.ReasoningText) != "" {
		t.Fatalf("reasoning must not be returned on failed turn, got %q", turn.ReasoningText)
	}
	if strings.Contains(turn.Content, "must-not-appear") {
		t.Fatal("forged chunk content must not be in turn result")
	}
}

func TestStreamAssistantTurnNormalizesOversizedInlineReasoningSpaces(t *testing.T) {
	chunk := `{"id":"chatcmpl-aaa","object":"chat.completion.chunk","created":1,"model":"test","choices":[{"index":0,"delta":{"reasoning_content":"**Gathering authoritative sources**\n\nI                                   need to collect...","content":"done"},"finish_reason":null}]}`
	client := mockStreamClient(t, "data: "+chunk+"\n\ndata: [DONE]\n\n")
	turn, err := llm.StreamAssistantTurn(context.Background(), client, openai.ChatCompletionNewParams{
		Model: "test",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hi"),
		},
	}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	want := "**Gathering authoritative sources**\n\nI need to collect..."
	if turn.ReasoningText != want {
		t.Fatalf("normalized reasoning mismatch:\n got: %q\nwant: %q", turn.ReasoningText, want)
	}
}
