package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
)

func readImageFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	var p, j bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 3))
	if err := png.Encode(&p, im); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&j, im, nil); err != nil {
		t.Fatal(err)
	}
	w, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"png": p.Bytes(), "jpeg": j.Bytes(), "webp": w}
}

func TestReadFileImagesFormatsAndSnapshot(t *testing.T) {
	for format, data := range readImageFixtures(t) {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			// Content detection must also work without a filename extension.
			path := filepath.Join(dir, "image")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]any{"path": path, "intent": "inspect image"})
			result, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: dir}, tooling.Invocation{Name: "readFile", Args: args})
			if err != nil {
				t.Fatal(err)
			}
			clean, attachments := images.TakeToolAttachments(result)
			if len(attachments) != 1 || attachments[0].MIMEType != "image/"+format || !bytes.Equal(attachments[0].Data, data) {
				t.Fatalf("incorrect image attachment for %s", format)
			}
			m := clean.(map[string]any)
			if m["type"] != "image" || m["width"].(int) < 1 || m["height"].(int) < 1 {
				t.Fatalf("metadata: %v", m)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			// Session replay must retain the exact image after the source disappears.
			wire, err := json.Marshal(chatstore.Message{Role: "tool", ToolImages: attachments})
			if err != nil {
				t.Fatal(err)
			}
			var restored chatstore.Message
			if err := json.Unmarshal(wire, &restored); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(restored.ToolImages[0].Data, data) {
				t.Fatal("snapshot lost on session round trip")
			}
		})
	}
}

func TestReadFileImagesRejectInvalidRangeAndOversize(t *testing.T) {
	pngData := readImageFixtures(t)["png"]
	for _, tc := range []struct {
		name  string
		data  []byte
		extra map[string]any
		want  string
	}{
		{"invalid", []byte("not a png"), nil, "invalid"},
		{"broken-header", []byte("\x89PNG\r\n\x1a\n"), nil, "invalid image"},
		{"range", pngData, map[string]any{"startLine": 1}, "only to text"},
		{"oversize", append(append([]byte{}, pngData...), make([]byte, 5*1024*1024)...), nil, "5 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "image.PNG"), tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			args := map[string]any{"path": "image.PNG", "intent": "inspect image"}
			for k, v := range tc.extra {
				args[k] = v
			}
			raw, _ := json.Marshal(args)
			_, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: dir}, tooling.Invocation{Name: "readFile", Args: raw})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestReadFileImagesOrchestratePersistsUnprintedImage(t *testing.T) {
	source := `package main
import "sdk"
func main() {
 r, err := sdk.ReadFileInfo("image.png", "inspect screenshot")
 if err != nil { panic(err) }
 if r.Type != "image" || r.MIMEType != "image/png" || r.Width != 2 || r.Height != 3 { panic("missing image metadata") }
}`
	args, _ := json.Marshal(map[string]string{"source": source, "intent": "read screenshot without printing"})
	backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, turns: []llm.AssistantTurnResult{
		{ToolCalls: []llm.AssistantToolCall{{ID: "image-read", Name: "orchestrate", Arguments: string(args)}}},
		{Content: "image received"},
	}}
	data := readImageFixtures(t)["png"]
	rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
		r.ToolOut = testToolOutputService(tooloutput.Limits{MaxBytes: 16, MaxLines: 1})
		if err := os.WriteFile(filepath.Join(r.ProjRoot, "image.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	})
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, msg := range rt.Session.Messages {
		if msg.Role != "tool" || msg.ToolCallID != "image-read" {
			continue
		}
		if len(msg.ToolImages) != 1 || !bytes.Equal(msg.ToolImages[0].Data, data) {
			t.Fatalf("image missing from tool result: %s", msg.Content)
		}
		if !strings.Contains(msg.Content, `"truncated":true`) {
			t.Fatal("test must exercise output truncation")
		}
		if strings.Contains(msg.Content, base64.StdEncoding.EncodeToString(data)) || strings.Contains(msg.Content, images.ToolAttachmentsKey) {
			t.Fatal("binary image leaked into textual tool output")
		}
		params := llm.MessageParams("", rt.Session.Messages, nil)
		wire, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(wire), "data:image/png;base64,"+base64.StdEncoding.EncodeToString(data)) {
			t.Fatal("image missing from provider request")
		}
		return
	}
	t.Fatal("orchestrate tool result missing")
}

func TestReadFileImagesOpenAIKeepsToolBatchTogether(t *testing.T) {
	data := readImageFixtures(t)["webp"]
	msgs := []chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "one", Name: "orchestrate", Arguments: "{}"}, {ID: "two", Name: "orchestrate", Arguments: "{}"}}},
		{Role: "tool", ToolCallID: "one", Content: "image", ToolImages: []images.Attachment{{Path: "a.webp", MIMEType: "image/webp", Data: data}}},
		{Role: "tool", ToolCallID: "two", Content: "text"},
		{Role: "assistant", Content: "done"},
	}
	params := llm.MessageParams("", msgs, nil)
	if len(params) != 6 || params[2].OfTool == nil || params[3].OfTool == nil || params[4].OfUser == nil || params[5].OfAssistant == nil {
		t.Fatal("image message interrupted assistant/tool reply ordering")
	}
	wire, err := json.Marshal(params[4])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), "data:image/webp;base64,"+base64.StdEncoding.EncodeToString(data)) {
		t.Fatal("missing WebP image content")
	}
}

func TestReadFileImagesAnthropicToolResultHTTP(t *testing.T) {
	for format, data := range readImageFixtures(t) {
		t.Run(format, func(t *testing.T) {
			srv := newAnthropicMockServer(t, "/v1/messages", func(r *http.Request) {
				var body struct {
					Messages []struct {
						Role    string
						Content []struct {
							Type      string
							ToolUseID string `json:"tool_use_id"`
							Content   []struct {
								Type   string
								Source struct {
									Type      string
									MediaType string `json:"media_type"`
									Data      string
								}
							}
						}
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if len(body.Messages) != 2 || len(body.Messages[1].Content) != 1 {
					t.Errorf("unexpected messages: %+v", body)
					return
				}
				result := body.Messages[1].Content[0]
				if result.Type != "tool_result" || result.ToolUseID != "image" || len(result.Content) != 3 {
					t.Errorf("unexpected tool result: %+v", result)
					return
				}
				img := result.Content[2]
				if img.Type != "image" || img.Source.Type != "base64" || img.Source.MediaType != "image/"+format || img.Source.Data != base64.StdEncoding.EncodeToString(data) {
					t.Errorf("incorrect %s image block", format)
				}
			}, anthropicSSEBody(), 0)
			defer srv.Close()
			backend := llm.NewAnthropicBackend(srv.URL, llm.AnthropicAuthFromAPIKey("test-key"))
			_, err := backend.StreamTurn(context.Background(), llm.TurnRequest{Model: "claude-test", Messages: []chatstore.Message{
				{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "image", Name: "orchestrate", Arguments: "{}"}}},
				{Role: "tool", ToolCallID: "image", Content: "image read", ToolImages: []images.Attachment{{Path: "image." + format, MIMEType: "image/" + format, Data: data}}},
			}}, io.Discard, llm.StreamOpts{})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
