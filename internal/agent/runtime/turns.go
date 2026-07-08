package agentruntime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/replcomplete"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/atmention"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/title"
)

func flushWriter(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}

func showGenerationStopped(out io.Writer) {
	termcolor.WriteSystem(out, "["+cliMsgGenerationStopped+"]")
	flushWriter(out)
}

func (r *Runtime) onUserMessage(ctx context.Context, line string, fromReadline bool) error {
	return r.onUserMessageWithAPIContent(ctx, line, "", fromReadline)
}

func (r *Runtime) onUserMessageWithAPIContent(ctx context.Context, line string, apiContent string, fromReadline bool) error {
	clean, _ := multiline.ParseMultilineControlRunes(line)
	line = multiline.TrimMessageEdges(clean)
	apiContent = multiline.TrimMessageEdges(apiContent)
	if config.NeedsOnboard(r.Cfg) || r.Prov == nil {
		return fmt.Errorf("config not set up; use /onboard")
	}
	if err := r.ensureReplSessionFileLock(); err != nil {
		return fmt.Errorf("session is locked by another process (solomon serve?): %w", err)
	}
	if r.ReplShellFirst {
		if strings.HasPrefix(line, "!") {
			line = multiline.TrimMessageEdges(strings.TrimPrefix(line, "!"))
			if line == "" {
				return nil
			}
		} else {
			if line == "" {
				return nil
			}
			return r.runUserShellLine(ctx, line)
		}
	} else if strings.HasPrefix(line, "!") {
		cmd := multiline.TrimMessageEdges(strings.TrimPrefix(line, "!"))
		if cmd == "" {
			return nil
		}
		return r.runUserShellLine(ctx, cmd)
	}
	var um chatstore.Message
	var firstUserLine string
	if strings.TrimSpace(apiContent) == "" && strings.Contains(line, "@") {
		entries, err := replcomplete.AtIndexEntries(ctx, replcomplete.ReplCompleteEnv{ProjRoot: r.ProjRoot})
		if err == nil {
			if exp, err := atmention.ExpandLine(ctx, line, r.ProjRoot, entries); err == nil && strings.TrimSpace(exp) != "" {
				apiContent = exp
			}
		}
	}
	r.mutateSession(func(s *chatstore.Session) {
		line = images.CanonicalizeUserLineForStorage(line, s.ImageFiles)
		if strings.TrimSpace(apiContent) != "" {
			apiContent = images.CanonicalizeUserLineForStorage(apiContent, s.ImageFiles)
		}
		if !r.EphemeralSession {
			r.markSessionFileCreated()
			if s.ID == "" && len(s.Messages) == 0 {
				s.ID = chatstore.NewPlaceholderChatID(time.Now())
			}
		}
		if r.EphemeralSession && s.Title == "" && len(s.Messages) == 0 {
			tSlug := title.NormalizeSlug(title.FallbackFromWords(line))
			s.Title = tSlug
			s.ID = chatstore.ChatIDHex(tSlug, s.CreatedAt)
			firstUserLine = strings.TrimSpace(line)
		}
		seq := checkpoint.Bump(s)
		um = chatstore.Message{Role: "user", Content: line, APIContent: apiContent}
		checkpoint.StampMsg(&um, s, seq)
		s.Messages = append(s.Messages, um)
		chatstore.RepairSessionMalformedImages(s)
		s.LastMessageAt = time.Now()
		s.LastUserMessageAt = time.Now()
	})
	if r.RL != nil {
		if err := r.ensureReplSessionFileLock(); err != nil {
			return fmt.Errorf("session is locked by another process (solomon serve?): %w", err)
		}
	}
	if firstUserLine != "" {
		go r.refineEphemeralTitle(ctx, firstUserLine)
	}
	if !r.machineMode() && !fromReadline {
		echoLine := termcolor.ColorizeAtTagsReplInput(termcolor.ColorizeImgTags(line))
		cpPref := checkpoint.FormatLinePrefix(um.CheckpointSeq, um.CheckpointBranchKey)
		youLbl := termcolor.WrapUser("You:")
		fmt.Fprintf(r.Out, "%s%s %s\n", cpPref, youLbl, echoLine)
	}
	if err := r.persistSession(); err != nil {
		logging.Log(logging.ERROR_LOG_LEVEL, "persist session failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		return err
	}
	if err := r.runAgentTurns(ctx); err != nil {
		return err
	}
	if !r.machineMode() {
		fmt.Fprintln(r.Out)
	}
	var deferTitle bool
	r.mutateSession(func(s *chatstore.Session) {
		deferTitle = !r.EphemeralSession && chatstore.IsPlaceholderChatID(s.ID)
	})
	if deferTitle {
		r.scheduleDeferredChatTitleFinalize(ctx)
	}
	return nil
}
