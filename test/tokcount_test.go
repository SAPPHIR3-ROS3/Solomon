package test

import (
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tokcount"
	"github.com/openai/openai-go/v2"
)

func TestTextTokens_nonEmpty(t *testing.T) {
	n := tokcount.TextTokens("hello world", tokcount.DefaultModel)
	if n <= 0 {
		t.Fatalf("TextTokens = %d, want > 0", n)
	}
}

func TestCountOpenAIMessages_overhead(t *testing.T) {
	one := tokcount.CountOpenAIMessages([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
	}, tokcount.DefaultModel)
	two := tokcount.CountOpenAIMessages([]openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("yo"),
	}, tokcount.DefaultModel)
	if one <= 0 || two <= one {
		t.Fatalf("counts should grow: one=%d two=%d", one, two)
	}
	delta := two - one
	yo := tokcount.TextTokens("yo", tokcount.DefaultModel)
	if delta < yo+3 {
		t.Fatalf("second message delta=%d, want at least yo+overhead=%d", delta, yo+3)
	}
}

func TestCountTools_simple(t *testing.T) {
	tools := []openai.ChatCompletionToolUnionParam{{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "grep",
				Description: openai.String("search files"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]any{
						"pattern": map[string]any{"type": "string", "description": "regex"},
					},
				},
			},
		},
	}}
	n := tokcount.CountTools(tools, tokcount.DefaultModel)
	if n <= 0 {
		t.Fatalf("CountTools = %d, want > 0", n)
	}
}

func TestCountTurnPrompt_matchesMessageParams(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "fix the bug"},
		{Role: "assistant", Content: "ok"},
	}
	req := llm.TurnRequest{
		System:   "you are helpful",
		Messages: msgs,
		Model:    tokcount.DefaultModel,
		Tools: []llm.ToolDef{{
			Name:        "read_file",
			Description: "read a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		}},
	}
	got := llm.CountTurnPrompt(req)
	wire := llm.MessageParams(req.System, req.Messages, req.ImageFiles)
	tools := make([]openai.ChatCompletionToolUnionParam, 0)
	for _, d := range req.Tools {
		props := d.Parameters
		if props == nil {
			props = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        d.Name,
					Description: openai.String(d.Description),
					Parameters:  openai.FunctionParameters(props),
				},
			},
		})
	}
	want := tokcount.CountOpenAIChatCompletion(wire, tools, req.Model)
	if got != want {
		t.Fatalf("CountTurnPrompt = %d, want %d", got, want)
	}
}

func TestPromptDisplaySplit_tokenWeights(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", Content: "short"},
		{Role: "assistant", Content: stringsRepeat("x", 200)},
		{Role: "user", Content: "last question"},
	}
	// Warm encoder cache; first split may load BPE tables.
	llm.PromptDisplaySplit("sys", msgs, 1000)
	start := time.Now()
	ctx, usr := llm.PromptDisplaySplit("sys", msgs, 1000)
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("PromptDisplaySplit took %v, want < 200ms", elapsed)
	}
	if ctx <= 0 || usr <= 0 {
		t.Fatalf("split ctx=%d usr=%d, both want > 0", ctx, usr)
	}
	if ctx+usr != 1000 {
		t.Fatalf("split sum %d != 1000", ctx+usr)
	}
	if usr >= ctx {
		t.Fatalf("last user should weigh less than context: ctx=%d usr=%d", ctx, usr)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
