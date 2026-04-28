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
		txt, err := r.streamNestedAssistant(roundCtx, system, msgs)
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
		transcript.WriteString(txt)
		transcript.WriteByte('\n')
		msgs = append(msgs, chatstore.Message{Role: "assistant", Content: txt})
		invs := tooling.ExtractToolInvocations(txt)
		if len(invs) == 0 {
			return transcript.String(), nil
		}
		for _, inv := range invs {
			res, err := r.execTool(ctx, inv)
			if err != nil {
				res = map[string]any{"error": err.Error()}
			}
			b, _ := json.Marshal(res)
			msgs = append(msgs, chatstore.Message{Role: "user", Content: "tool_result(" + string(b) + ")"})
		}
	}
	return transcript.String(), nil
}

func (r *Runtime) streamNestedAssistant(ctx context.Context, system string, msgs []chatstore.Message) (string, error) {
	p := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(r.Model),
		Messages: llm.MessageParams(system, msgs),
	}
	llm.ApplyMaxResponseTokens(r.Cfg, &p)
	fmt.Fprintf(r.Out, "%s%s(subagent):%s ", termcolor.Assistant, r.Model, termcolor.Reset)
	return llm.StreamText(ctx, r.Client, p, termcolor.NewToolLineWriter(r.Out), llm.StreamOpts{ShowThinking: r.Cfg.ShowThinking, ReasoningSink: r.Out})
}

func (r *Runtime) summarizeNested(ctx context.Context, msgs []chatstore.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(m.Role + ": " + m.Content + "\n")
	}
	p := openai.ChatCompletionNewParams{
		Model: shared.ChatModel(r.Model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("Briefly summarize the following conversation turns."),
			openai.UserMessage(sb.String()),
		},
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
