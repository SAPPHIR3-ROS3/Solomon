//go:build !windows

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/creack/pty"
)

type imageTerminalCapture struct {
	bytes.Buffer
	fd uintptr
}

func (w *imageTerminalCapture) Fd() uintptr { return w.fd }
func TestTerminalImageOutputLiveReplayAndMachine(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("SOLOMON_IMAGE_COLOR", "auto")
	t.Setenv("SOLOMON_IMAGE_PREVIEW", "")
	const content = "Here is the result: ![preview](result.png)"
	for _, machine := range []bool{false, true} {
		name := "human"
		if machine {
			name = "machine"
		}
		t.Run(name, func(t *testing.T) {
			out := &imageTerminalCapture{fd: slave.Fd()}
			backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, turns: []llm.AssistantTurnResult{{Content: content}}}
			rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
				r.Out = out
				if machine {
					r.EventSink = cievents.NewJSONLEmitter(out)
				}
				if err := os.WriteFile(filepath.Join(r.ProjRoot, "result.png"), readImageFixtures(t)["png"], 0600); err != nil {
					t.Fatal(err)
				}
			})
			if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
				t.Fatal(err)
			}
			if rt.Session.Messages[len(rt.Session.Messages)-1].Content != content {
				t.Fatal("preview changed persisted content")
			}
			assistant := rt.Session.Messages[len(rt.Session.Messages)-1]
			if len(assistant.MediaPreviews) != 1 {
				t.Fatal("assistant image snapshot missing")
			}
			if err := os.Remove(filepath.Join(rt.ProjRoot, "result.png")); err != nil {
				t.Fatal(err)
			}
			replay := &imageTerminalCapture{fd: slave.Fd()}
			commands.WriteLabeledTranscript(replay, []chatstore.Message{assistant}, "test", false)
			if strings.Contains(replay.String(), "unavailable") || !strings.Contains(replay.String(), "38;2;") {
				t.Fatal("assistant snapshot replay failed", replay.String())
			}
			wire, _ := json.Marshal(llm.MessageParams("", []chatstore.Message{assistant}, nil))
			if strings.Contains(string(wire), "data:image/") {
				t.Fatal("presentation preview became a model input")
			}

			if machine {
				for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
					if !json.Valid([]byte(line)) {
						t.Fatalf("preview contaminated JSONL: %q", line)
					}
				}
				if strings.Contains(out.String(), "Image:") || strings.Contains(out.String(), "\x1b") {
					t.Fatal("human preview leaked into machine output")
				}
			} else if !strings.Contains(out.String(), "Image: "+filepath.Join(rt.ProjRoot, "result.png")) || !strings.Contains(out.String(), "38;2;") {
				t.Fatalf("live preview missing through output mux: %q", out.String())
			}
		})
	}
	// Transcript attachments are rendered from snapshot bytes without the source.
	out := &imageTerminalCapture{fd: slave.Fd()}
	commands.WriteLabeledTranscript(out, []chatstore.Message{{Role: "tool", Content: `{"type":"image","path":"/deleted.png"}`, ToolImages: []images.Attachment{{Path: "/deleted.png", MIMEType: "image/png", Data: readImageFixtures(t)["png"]}}}}, "test", false)
	if strings.Count(out.String(), "Image:") != 1 || strings.Contains(out.String(), "unavailable") || !strings.Contains(out.String(), "38;2;") {
		t.Fatal("snapshot replay failed", out.String())
	}
}
