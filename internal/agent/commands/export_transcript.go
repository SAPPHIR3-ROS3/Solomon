package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

type markdownExportMeta struct {
	Title           string
	ExportedAt      time.Time
	ChatID          string
	ProjectRoot     string
	ProjectHex      string
	Model           string
	Provider        string
	Mode            string
	CreatedAt       time.Time
	LastMessageAt   time.Time
	MessageCount    int
	Ephemeral       bool
	ExportRoot      string
	ExportRootIsCfg bool
}

func writeMarkdownExport(w io.Writer, meta markdownExportMeta, sess *chatstore.Session, showUsage bool) error {
	if sess == nil {
		return fmt.Errorf("nil session")
	}
	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = "Untitled chat"
	}
	fmt.Fprintf(w, "# %s\n\n", title)
	writeMarkdownExportHeader(w, meta)
	fmt.Fprintln(w)
	msgs := append([]chatstore.Message(nil), sess.Messages...)
	if showUsage {
		chatstore.BackfillSessionAssistantUsage(msgs)
	}
	writeMarkdownTranscript(w, msgs, meta.Model, showUsage)
	lines, err := exportImageAppendixLines(sess)
	if err != nil {
		return err
	}
	if len(lines) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Images")
		for _, line := range lines {
			fmt.Fprintln(w, line)
		}
	}
	return nil
}

func writeMarkdownExportHeader(w io.Writer, meta markdownExportMeta) {
	fmt.Fprintln(w, "## Metadata")
	fmt.Fprintf(w, "- **Exported:** %s\n", meta.ExportedAt.UTC().Format(time.RFC3339))
	if meta.ChatID != "" {
		fmt.Fprintf(w, "- **Chat ID:** %s\n", meta.ChatID)
	}
	if meta.ProjectRoot != "" {
		fmt.Fprintf(w, "- **Project:** %s\n", meta.ProjectRoot)
	}
	if meta.ProjectHex != "" {
		fmt.Fprintf(w, "- **Project hex:** %s\n", meta.ProjectHex)
	}
	if meta.Provider != "" {
		fmt.Fprintf(w, "- **Provider:** %s\n", meta.Provider)
	}
	if meta.Model != "" {
		fmt.Fprintf(w, "- **Model:** %s\n", meta.Model)
	}
	if meta.Mode != "" {
		fmt.Fprintf(w, "- **Mode:** %s\n", meta.Mode)
	}
	if !meta.CreatedAt.IsZero() {
		fmt.Fprintf(w, "- **Created:** %s\n", meta.CreatedAt.UTC().Format(time.RFC3339))
	}
	if !meta.LastMessageAt.IsZero() {
		fmt.Fprintf(w, "- **Last message:** %s\n", meta.LastMessageAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(w, "- **Messages:** %d\n", meta.MessageCount)
	if meta.Ephemeral {
		fmt.Fprintln(w, "- **Ephemeral session:** yes")
	}
	rootNote := meta.ExportRoot
	if !meta.ExportRootIsCfg {
		rootNote += " (default)"
	}
	fmt.Fprintf(w, "- **Export root:** %s\n", rootNote)
}

func writeMarkdownTranscript(out io.Writer, msgs []chatstore.Message, model string, showUsage bool) {
	fmt.Fprintln(out, "## Transcript")
	turnStart := 0
	for i := range msgs {
		m := msgs[i]
		if m.Role == "user" && !strings.HasPrefix(strings.TrimSpace(m.Content), "tool_result(") {
			if showUsage && i > turnStart {
				writeMarkdownUsageLine(out, msgs, turnStart, i)
			}
			writeMarkdownMessage(out, msgs, i, model)
			turnStart = i + 1
			continue
		}
		writeMarkdownMessage(out, msgs, i, model)
	}
	if showUsage && turnStart < len(msgs) {
		writeMarkdownUsageLine(out, msgs, turnStart, len(msgs))
	}
}

func writeMarkdownUsageLine(out io.Writer, msgs []chatstore.Message, start, end int) {
	ctxTok, usrTok, reasonTok, respTok, totalTok, outputTPS, ttftSecs, promptTPS, turnWallSecs, ctxEst, ok := chatstore.StoredUsageLineForTurnRange(msgs, start, end)
	if !ok {
		return
	}
	fmt.Fprintf(out, "_%s_\n\n", exportPlainUsageLine(ctxTok, usrTok, reasonTok, respTok, totalTok, outputTPS, ttftSecs, promptTPS, ctxEst, turnWallSecs))
}

func exportPlainUsageLine(contextPromptTok, lastUserPromptTok, reasoningTokens, responseTokens, totalTokens int64, outputTPS, ttftSecs, promptTPS float64, contextEstimated bool, turnWallSecs float64) string {
	formatContext := func(n int64) string {
		s := strconv.FormatInt(n, 10)
		if contextEstimated && n > 0 {
			return "~" + s
		}
		return s
	}
	var promptSeg string
	switch {
	case contextPromptTok <= 0 && lastUserPromptTok <= 0:
		promptSeg = "0"
	case lastUserPromptTok <= 0:
		promptSeg = formatContext(contextPromptTok)
	case contextPromptTok <= 0:
		promptSeg = strconv.FormatInt(lastUserPromptTok, 10)
	default:
		promptSeg = formatContext(contextPromptTok) + "+" + strconv.FormatInt(lastUserPromptTok, 10)
	}
	line := "token: " + promptSeg + "+" +
		strconv.FormatInt(reasoningTokens, 10) + "+" +
		strconv.FormatInt(responseTokens, 10) + "=" +
		strconv.FormatInt(totalTokens, 10) +
		fmt.Sprintf("\t%st/s ttft:%ss pp:%st/s", formatExportFloatMax3(outputTPS), formatExportFloatMax3(ttftSecs), formatExportFloatMax3(promptTPS))
	line += "\t worked for " + formatExportWorkedDuration(turnWallSecs)
	return line
}

func formatExportFloatMax3(f float64) string {
	s := fmt.Sprintf("%.3f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func formatExportWorkedDuration(secs float64) string {
	if secs <= 0 {
		return "0s"
	}
	h := int(secs / 3600)
	r1 := secs - float64(h*3600)
	m := int(r1 / 60)
	s := r1 - float64(m*60)
	var b strings.Builder
	if h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m > 0 || h > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	fmt.Fprintf(&b, "%ss", formatExportFloatMax3(s))
	return b.String()
}

func writeMarkdownMessage(out io.Writer, msgs []chatstore.Message, idx int, model string) {
	m := msgs[idx]
	cpTag := ""
	if chatstore.MessageCheckpointTagVisible(m) {
		cpTag = checkpoint.FormatCheckpointTag(m.CheckpointSeq, m.CheckpointBranchKey)
	}
	if cpTag != "" {
		fmt.Fprintf(out, "### Checkpoint %s\n\n", cpTag)
	}
	switch m.Role {
	case "user":
		if strings.HasPrefix(m.Content, "tool_result(") {
			return
		}
		fmt.Fprintf(out, "**You:**\n\n%s\n\n", strings.TrimSpace(m.Content))
	case "assistant":
		if strings.Contains(m.Content, "[Conversation summary]") {
			fmt.Fprintf(out, "**Summary:**\n\n```\n%s\n```\n\n", strings.TrimSpace(m.Content))
			break
		}
		rtxt, cshow := chatstore.AssistantDisplayParts(m)
		rtxt = chatstore.StripAllImgPlaceholderLiterals(tooling.StripLegacyToolBlocks(rtxt))
		cshow = chatstore.StripAllImgPlaceholderLiterals(tooling.LegacyProseOutsideToolCalls(cshow))
		if rtxt != "" {
			fmt.Fprintf(out, "> %s\n\n", strings.ReplaceAll(strings.TrimSpace(rtxt), "\n", "\n> "))
		}
		if cshow != "" {
			label := strings.TrimSpace(model)
			if label == "" {
				label = "Assistant"
			}
			fmt.Fprintf(out, "**%s:**\n\n%s\n\n", label, strings.TrimSpace(cshow))
		}
		toolCalls := m.ToolCalls
		if len(toolCalls) == 0 {
			if invs, err := tooling.ExtractToolInvocations(m.Content); err == nil && len(invs) > 0 {
				for _, inv := range invs {
					toolCalls = append(toolCalls, chatstore.ToolCall{Name: inv.Name, Arguments: string(inv.Args)})
				}
			}
		}
		for _, tc := range toolCalls {
			cpSeq, branch := m.CheckpointSeq, m.CheckpointBranchKey
			if tc.CpSeqSet {
				cpSeq, branch = tc.CheckpointSeq, tc.CheckpointBranchKey
			}
			if intent := tooling.ExtractToolIntent(json.RawMessage(tc.Arguments)); intent != "" {
				tag := checkpoint.FormatCheckpointTag(cpSeq, branch)
				if tag != "" {
					fmt.Fprintf(out, "_%s intent:_ %s\n\n", tag, intent)
				} else {
					fmt.Fprintf(out, "_intent:_ %s\n\n", intent)
				}
			}
			for _, line := range exportPlainToolLines(tc.Name, json.RawMessage(tc.Arguments)) {
				fmt.Fprintf(out, "%s\n\n", line)
			}
		}
	case "tool":
		toolName := toolNameForResult(msgs, idx)
		for _, line := range exportPlainToolResultLines(toolName, m.Content) {
			fmt.Fprintf(out, "%s\n\n", line)
		}
	case "system":
		fmt.Fprintf(out, "_system:_ %s\n\n", strings.TrimSpace(m.Content))
	}
}

func exportPlainToolLines(name string, rawArgs json.RawMessage) []string {
	lines := tooling.FormatToolDisplayLines(name, rawArgs)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = exportStripANSI(line)
		if line == "" {
			continue
		}
		out = append(out, "**"+line+"**")
	}
	return out
}

func exportPlainToolResultLines(toolName, payload string) []string {
	lines := tooling.FormatToolResultDisplayLines(toolName, payload)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = exportStripANSI(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func exportStripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inEsc := false
	for i := 0; i < len(s); i++ {
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func exportImageAppendixLines(sess *chatstore.Session) ([]string, error) {
	if sess == nil || len(sess.ImageFiles) == 0 {
		return nil, nil
	}
	seqs := make([]int, 0, len(sess.ImageFiles))
	for seq := range sess.ImageFiles {
		seqs = append(seqs, seq)
	}
	sort.Ints(seqs)
	lines := make([]string, 0, len(seqs))
	for _, seq := range seqs {
		path := strings.TrimSpace(sess.ImageFiles[seq])
		if path == "" {
			continue
		}
		data, err := exportReadImageDataURI(path)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", images.Placeholder(seq), err)
		}
		lines = append(lines, fmt.Sprintf("%s = %s", images.Placeholder(seq), data))
	}
	return lines, nil
}

func exportReadImageDataURI(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := exportDetectImageMIME(b)
	encoded := base64.StdEncoding.EncodeToString(b)
	return "data:" + mime + ";base64," + encoded, nil
}

func exportDetectImageMIME(b []byte) string {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}):
		return "image/png"
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return "image/jpeg"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func writeExportFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
