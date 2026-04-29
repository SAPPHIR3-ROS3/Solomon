package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/llm"
	"solomon/internal/termcolor"
	"solomon/internal/tooling"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/openai/openai-go/v2/shared"
)

func (r *Runtime) runNested(ctx context.Context, task string) (string, error) {
	sys, err := r.systemPrompt()
	if err != nil {
		return "", err
	}
	return r.runNestedWithSystem(ctx, sys, task)
}

func (r *Runtime) runNestedWithSystem(ctx context.Context, system, task string) (string, error) {
	msgs := []chatstore.Message{{Role: "user", Content: task}}
	var transcript strings.Builder

	for iteration := 0; iteration < 512; iteration++ {
		dur := time.Duration(config.SubagentTimeout(r.Cfg)) * time.Minute
		roundCtx, cancel := context.WithDeadline(ctx, time.Now().Add(dur))
		turn, err := r.streamNestedAssistant(roundCtx, system, msgs)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) {
			sum, _ := r.summarizeNested(ctx, msgs)
			fmt.Fprintf(r.Out, "\n%s\nSubagent paused (timeout).\nContinue? [y/N]: ", sum)
			br := bufio.NewReader(os.Stdin)
			line, _ := br.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "y" {
				return transcript.String(), nil
			}
			continue
		}
		if err != nil {
			return transcript.String(), err
		}
		transcript.WriteString(turn.Content)
		transcript.WriteByte('\n')
		ast := chatstore.Message{Role: "assistant", Content: turn.Content}
		for _, tc := range turn.ToolCalls {
			ast.ToolCalls = append(ast.ToolCalls, chatstore.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		msgs = append(msgs, ast)
		var invs []tooling.Invocation
		var toolIDs []string
		if len(turn.ToolCalls) > 0 {
			for _, tc := range turn.ToolCalls {
				invs = append(invs, tooling.Invocation{Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
				toolIDs = append(toolIDs, tc.ID)
			}
		} else if r.Session.LegacyTools {
			for _, inv := range tooling.ExtractToolInvocations(turn.Content) {
				invs = append(invs, inv)
				toolIDs = append(toolIDs, "")
			}
		}
		if len(invs) == 0 {
			return transcript.String(), nil
		}
		for i, inv := range invs {
			r.printToolLine(inv.Name, inv.Args)
			transcript.WriteString(fmt.Sprintf("Tool: %s(%s)\n", inv.Name, string(inv.Args)))
			res, err := r.execTool(ctx, inv)
			if err != nil {
				res = map[string]any{"error": err.Error()}
			}
			b, err := json.Marshal(res)
			if err != nil {
				b = []byte(`{"error":"marshal"}`)
			}
			payload := string(b)
			if id := toolIDs[i]; id != "" {
				msgs = append(msgs, chatstore.Message{Role: "tool", ToolCallID: id, Content: payload})
			} else {
				msgs = append(msgs, chatstore.Message{Role: "user", Content: "tool_result(" + payload + ")"})
			}
		}
	}
	return transcript.String(), nil
}

func (r *Runtime) streamNestedAssistant(ctx context.Context, system string, msgs []chatstore.Message) (llm.AssistantTurnResult, error) {
	tools, err := NativeToolParams(r.Mode)
	if err != nil {
		return llm.AssistantTurnResult{}, err
	}
	p := openai.ChatCompletionNewParams{
		Model:             shared.ChatModel(r.Model),
		Messages:          llm.MessageParams(system, msgs),
		ReasoningEffort:   shared.ReasoningEffort("none"),
		Tools:             tools,
		ParallelToolCalls: param.NewOpt(true),
	}
	llm.ApplyMaxResponseTokens(r.Cfg, &p)
	fmt.Fprintf(r.Out, "%s%s(subagent):%s ", termcolor.Assistant, r.Model, termcolor.Reset)
	turn, err := llm.StreamAssistantTurn(ctx, r.Client, p, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
	if err != nil {
		return turn, err
	}
	fmt.Fprintln(r.Out)
	if r.Cfg.UsageStatsEnabled() {
		fmt.Fprintln(r.Out, termcolor.UsageTokensLine(turn.Usage.PromptTokens, turn.Usage.ReasoningTokens, turn.Usage.ResponseTokens, turn.Usage.TotalTokens, turn.Usage.OutputTPS, turn.Usage.TTFTSecs, turn.Usage.PromptTPS))
	}
	return turn, nil
}

func (r *Runtime) summarizeNested(ctx context.Context, msgs []chatstore.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role + ": " + m.Content + "\n")
	}
	p := openai.ChatCompletionNewParams{
		Model:           shared.ChatModel(r.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Briefly summarize the following conversation turns."),
			openai.UserMessage(sb.String()),
		},
		ReasoningEffort: shared.ReasoningEffort("none"),
	}
	llm.ApplyMaxResponseTokens(r.Cfg, &p)
	resp, err := r.Client.Chat.Completions.New(ctx, p)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no summary choices")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
