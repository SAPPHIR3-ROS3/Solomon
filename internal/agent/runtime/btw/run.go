package btw

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

const CatchUpPause = 20

type Host interface {
	SessionMessagesSnapshot() ([]chatstore.Message, map[int]string)
	SystemPromptBtw(disableThinking bool) (string, error)
	BtwLinePrefixes() (userPrefix, assistantPrefix string)
	ReadBtwInput(out io.Writer, userPrefix, initial string) (string, error)
	Config() *config.Root
	ModelName() string
	Backend() llm.CompletionBackend
}

func btwPrompts(transcript, question, language string, disableThinking bool, sysFn func(bool) (string, error)) (string, string, error) {
	sys, err := sysFn(disableThinking)
	if err != nil {
		return "", "", err
	}
	user, err := prompt.RenderBtw(prompt.BtwData{
		Transcript:      transcript,
		Question:        question,
		Language:        language,
		DisableThinking: disableThinking,
	})
	if err != nil {
		user = transcript + "\n\nQuestion:\n" + question
	}
	return sys, user, nil
}

func Execute(ctx context.Context, h Host, out io.Writer, question, assistantPrefix string) error {
	question = strings.TrimSpace(question)
	if question == "" {
		return fmt.Errorf("empty /btw question")
	}
	msgs, _ := h.SessionMessagesSnapshot()
	msgs = CompleteMessages(msgs)
	if len(msgs) == 0 {
		return fmt.Errorf("no conversation context for /btw")
	}
	cfg := h.Config()
	transcript := commands.FormatChatTranscript(msgs)
	disableThinking := cfg.ReasoningEffortIsNone()
	sys, user, err := btwPrompts(transcript, question, cfg.EffectiveResponseLanguage(), disableThinking, h.SystemPromptBtw)
	if err != nil {
		return err
	}
	if h.Backend() == nil {
		return fmt.Errorf("LLM backend not configured")
	}
	fmt.Fprintf(out, "%s%s: ", assistantPrefix, termcolor.WrapAssistant(h.ModelName()))
	_, _, err = h.Backend().StreamText(ctx, llm.SimpleCompletionRequest{
		Cfg:                   cfg,
		Model:                 h.ModelName(),
		System:                sys,
		User:                  user,
		ForceDisableReasoning: disableThinking,
	}, out, llm.StreamOpts{ShowThinking: cfg.ShowThinking, ReasoningSink: out})
	if err != nil {
		return err
	}
	fmt.Fprintln(out)
	termcolor.PrintBtwSeparator(out)
	return nil
}
