package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
)

func Threshold(d Deps, parts []string) error {
	if len(parts) < 2 {
		PrintSystemf(d.Out, "compaction_threshold_tokens=%d (auto /summarize when prompt_tokens >= this after an assistant reply; usage must be reported by the API)", d.CompactionThresholdTokens())
		return nil
	}
	n, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return err
	}
	if n < config.MinCompactionThresholdTokens {
		return fmt.Errorf("threshold must be >= %d", config.MinCompactionThresholdTokens)
	}
	d.SetCompactionThresholdTokens(n)
	PrintSystemf(d.Out, "compaction_threshold_tokens=%d", n)
	return nil
}

func FormatChatTranscript(msgs []chatstore.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "User:\n%s\n\n", m.Content)
		case "assistant":
			_, cshow := chatstore.AssistantDisplayParts(m)
			if cshow != "" {
				fmt.Fprintf(&b, "Assistant:\n%s\n\n", cshow)
			}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					fmt.Fprintf(&b, "  [tool_call %s] %s(%s)\n", tc.ID, tc.Name, tc.Arguments)
				}
				fmt.Fprintf(&b, "\n")
			}
		case "tool":
			fmt.Fprintf(&b, "Tool[%s]:\n%s\n\n", m.ToolCallID, m.Content)
		default:
			fmt.Fprintf(&b, "%s:\n%s\n\n", m.Role, m.Content)
		}
	}
	return b.String()
}

func summarizePromptFromTemplate(transcript string, disableThinking bool) (string, string) {
	d := prompt.SummarizeData{Transcript: transcript, DisableThinking: disableThinking}
	sys, err := prompt.RenderSummarizeSystem(d)
	if err != nil {
		sys = prompt.SystemWithNoThink(disableThinking, prompt.SummarizeSystemFallback())
	}
	user, err := prompt.RenderSummarize(d)
	if err != nil {
		return sys, transcript
	}
	return sys, user
}

func summarizeToolObjective(tc chatstore.ToolCall) string {
	name := strings.TrimSpace(tc.Name)
	if name == "" {
		name = "tool"
	}
	args := strings.TrimSpace(compactJSONArgs(tc.Arguments))
	if args == "" {
		return fmt.Sprintf("Use %s to advance the task.", name)
	}
	return fmt.Sprintf("Use %s to advance the task with args: %s", name, truncateRunes(args, 140))
}

func appendRetainedEntry(entries *[]chatstore.Message, role, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if role == "assistant" && len(*entries) > 0 && (*entries)[len(*entries)-1].Role == "assistant" {
		(*entries)[len(*entries)-1].Content = strings.TrimSpace((*entries)[len(*entries)-1].Content + "\n\n" + content)
		return
	}
	*entries = append(*entries, chatstore.Message{Role: role, Content: content})
}

func FormatRetainedMessages(msgs []chatstore.Message) string {
	var entries []chatstore.Message
	for _, m := range msgs {
		switch m.Role {
		case "user":
			if strings.HasPrefix(strings.TrimSpace(m.Content), "tool_result(") {
				continue
			}
			appendRetainedEntry(&entries, "user", m.Content)
		case "assistant":
			var parts []string
			_, cshow := chatstore.AssistantDisplayParts(m)
			if cshow != "" {
				parts = append(parts, cshow)
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, "Tool objective: "+summarizeToolObjective(tc))
			}
			appendRetainedEntry(&entries, "assistant", strings.Join(parts, "\n\n"))
		case "tool":
			continue
		default:
			appendRetainedEntry(&entries, m.Role, m.Content)
		}
	}
	var b strings.Builder
	for _, m := range entries {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "User:\n%s\n\n", m.Content)
		case "assistant":
			fmt.Fprintf(&b, "Assistant:\n%s\n\n", m.Content)
		default:
			fmt.Fprintf(&b, "%s:\n%s\n\n", m.Role, m.Content)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func CompactSummaryBody(sep, summaryLLM, retainedBlock string) string {
	summaryLLM = strings.TrimSpace(summaryLLM)
	retainedBlock = strings.TrimSpace(retainedBlock)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n\n%s", sep, "[Conversation summary]", sep, summaryLLM)
	if retainedBlock != "" {
		fmt.Fprintf(&b, "\n\n%s\n%s\n%s\n\n%s", sep, "[Retained messages]", sep, retainedBlock)
		fmt.Fprintf(&b, "\n%s", sep)
	} else {
		fmt.Fprintf(&b, "\n%s", sep)
	}
	return chatstore.NormalizeSummaryWhitespace(b.String())
}

// RenderCompactSummaryBody applies terminal color to a plain-text summary body
// for display only. The returned string must never be persisted.
func RenderCompactSummaryBody(body string) string {
	return termcolor.WrapContext(body)
}

// SummarizeProgressLine returns the progress line text for a given number of dots.
func SummarizeProgressLine(dots int) string {
	return "Summarizing" + strings.Repeat(".", dots)
}

// summarizeProgress manages a live same-line progress indicator.
// It prints "Summarizing" immediately and appends one dot every 5 seconds on the same line.
// Call stop() exactly once when summarization finishes or fails; it blocks until the
// goroutine has printed its final line, preventing output interleaving.
type summarizeProgress struct {
	out     io.Writer
	done    chan struct{}
	stopped chan struct{}
}

type SummarizeProgress struct {
	p *summarizeProgress
}

func NewSummarizeProgress(out io.Writer) *SummarizeProgress {
	p := &summarizeProgress{out: out, done: make(chan struct{}), stopped: make(chan struct{})}
	fmt.Fprint(p.out, SummarizeProgressLine(0))
	go p.run()
	return &SummarizeProgress{p: p}
}

func (s *SummarizeProgress) Stop() {
	s.p.stop()
}

func (p *summarizeProgress) run() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer close(p.stopped)
	dots := 0
	for {
		select {
		case <-p.done:
			fmt.Fprintf(p.out, "\r%s\n", SummarizeProgressLine(dots))
			return
		case <-ticker.C:
			dots++
			fmt.Fprintf(p.out, "\r%s", SummarizeProgressLine(dots))
		}
	}
}

func (p *summarizeProgress) stop() {
	close(p.done)
	<-p.stopped
}

func SummarizeBody(d Deps) (string, error) {
	var msgs []chatstore.Message
	if d.MutateSession != nil {
		d.MutateSession(func(sess *chatstore.Session) {
			if sess != nil {
				msgs = append([]chatstore.Message(nil), sess.Messages...)
			}
		})
	} else if sess := d.Session(); sess != nil {
		msgs = append([]chatstore.Message(nil), sess.Messages...)
	}
	if len(msgs) == 0 {
		return "", fmt.Errorf("no messages to summarize")
	}
	progress := NewSummarizeProgress(d.Out)
	transcript := FormatChatTranscript(msgs)
	sys, userPrompt := summarizePromptFromTemplate(transcript, d.Cfg.ReasoningEffortIsNone())
	const sep = "================================================================================"
	if d.Backend == nil {
		return "", fmt.Errorf("LLM backend not configured")
	}
	summary, usage, err := d.Backend.StreamText(d.Ctx, llm.SimpleCompletionRequest{
		Cfg:                   d.Cfg,
		Model:                 d.Model(),
		System:                sys,
		User:                  userPrompt,
		ForceDisableReasoning: false,
	}, io.Discard, llm.StreamOpts{})
	progress.Stop()
	if err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "/summarize StreamText failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return "", err
	}
	if d.Cfg.UsageStatsEnabled() {
		ms := []chatstore.Message{{Role: "user", Content: transcript}}
		ctxTok, usrTok, ctxEst := llm.UsagePromptParts(sys, ms, usage.PromptTokens, usage.CachedPromptTokens)
		fmt.Fprintln(d.Out, termcolor.UsageTokensLine(ctxTok, usrTok, usage.ReasoningTokens, usage.ResponseTokens, usage.TotalTokens, usage.OutputTPS, usage.TTFTSecs, usage.PromptTPS, ctxEst, usage.TurnWallSecs))
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		logging.Log(logging.WARNING_LOG_LEVEL, "/summarize empty summary from model")
		return "", fmt.Errorf("empty summary from model")
	}
	var retainedBlock string
	if len(msgs) > 8 {
		retainedBlock = FormatRetainedMessages(msgs[len(msgs)-8:])
	} else {
		retainedBlock = FormatRetainedMessages(msgs)
	}
	body := CompactSummaryBody(sep, summary, retainedBlock)
	return chatstore.ScrubCompactSummaryContent(body), nil
}

func Summarize(d Deps) error {
	body, err := SummarizeBody(d)
	if err != nil {
		return err
	}
	if d.MutateSession == nil {
		return fmt.Errorf("no active session")
	}
	d.MutateSession(func(sess *chatstore.Session) {
		sess.Messages = []chatstore.Message{{Role: "assistant", Content: body}}
		sess.MainOrphans = nil
		sess.CheckpointBranchSuffix = ""
		sess.ForkChildCount = nil
		sess.CheckpointLast = -1
		sess.CheckpointCP0 = true
		sess.LastCommitOID = ""
		sess.LastMessageAt = time.Now()
		chatstore.RepairSessionMalformedImages(sess)
	})
	if d.PersistSession == nil {
		return fmt.Errorf("no active session")
	}
	if err := d.PersistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "/summarize persist compacted session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	fmt.Fprint(d.Out, "\033[2J\033[H")
	fmt.Fprintln(d.Out, RenderCompactSummaryBody(body))
	PrintSystem(d.Out, "History compacted: summary saved; previous messages have been replaced.")
	return nil
}
