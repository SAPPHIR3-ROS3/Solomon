package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	solomonmcp "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mcp"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooloutput"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMediaArtifactsMCPOrchestrateSurviveTruncation(t *testing.T) {
	data := readImageFixtures(t)["png"]
	root := t.TempDir()
	poster := filepath.Join(root, "poster.png")
	if err := os.WriteFile(poster, data, 0600); err != nil {
		t.Fatal(err)
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "visual-media-test", Version: "1"}, nil)
	server.AddTool(&sdkmcp.Tool{Name: "make_image", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.ImageContent{MIMEType: "image/png", Data: data}}}, nil
	})
	server.AddTool(&sdkmcp.Tool{Name: "make_video", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{StructuredContent: map[string]any{"type": "video", "path": filepath.Join(root, "clip.mp4"), "poster_path": poster}, Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "video generated"}}}, nil
	})
	httpServer := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, &sdkmcp.StreamableHTTPOptions{Stateless: true}))
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mgr := solomonmcp.NewManagerWithOptions(ctx, &solomonmcp.Config{Servers: []solomonmcp.ServerConfig{{Name: "visual", Type: solomonmcp.TransportStreamableHTTP, URL: httpServer.URL}}}, nil, &solomonmcp.ManagerOptions{DisableCatalogSubscriptions: true})
	defer mgr.Close()
	if _, _, err := mgr.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	source := `package main
import "sdk"
func main(){
 if _,err:=sdk.mcp.make_image("create image",map[string]any{});err!=nil{panic(err)}
 if _,err:=sdk.mcp.make_video("create video",map[string]any{});err!=nil{panic(err)}
}`
	args, _ := json.Marshal(map[string]string{"source": source, "intent": "produce visual artifacts without printing their metadata"})
	backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, turns: []llm.AssistantTurnResult{{ToolCalls: []llm.AssistantToolCall{{ID: "media", Name: "orchestrate", Arguments: string(args)}}}, {Content: "done"}}}
	rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
		r.MCP = mgr
		r.ToolOut = testToolOutputService(tooloutput.Limits{MaxBytes: 16, MaxLines: 1})
	})
	if err := rt.RunAgentTurnsForTest(ctx); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range rt.Session.Messages {
		if m.Role != "tool" || m.ToolCallID != "media" {
			continue
		}
		found = true
		if len(m.ToolImages) != 2 {
			t.Fatalf("lost unprinted media: %s, snapshots=%d", m.Content, len(m.ToolImages))
		}
		if !bytes.Contains([]byte(m.Content), []byte(`"truncated":true`)) {
			t.Fatal("test must exercise text truncation")
		}
		if bytes.Contains([]byte(m.Content), []byte(base64.StdEncoding.EncodeToString(data))) || bytes.Contains([]byte(m.Content), []byte(images.ToolAttachmentsKey)) {
			t.Fatal("binary leaked into textual result")
		}
		video := false
		for _, a := range m.ToolImages {
			if !bytes.Equal(a.Data, data) {
				t.Fatal("snapshot differs")
			}
			if a.MediaType == "video" {
				video = true
				if a.Path != filepath.Join(root, "clip.mp4") || a.PreviewPath != poster {
					t.Fatal("original video provenance lost")
				}
			}
		}
		if !video {
			t.Fatal("video poster was not captured")
		}
	}
	if !found {
		t.Fatal("missing orchestrate result")
	}
}

func TestMediaPresentationSnapshotsAreNotModelInputs(t *testing.T) {
	data := readImageFixtures(t)["png"]
	messages := []chatstore.Message{
		{Role: "assistant", ToolCalls: []chatstore.ToolCall{{ID: "media", Name: "orchestrate", Arguments: "{}"}}},
		{Role: "tool", ToolCallID: "media", Content: `{"type":"video","path":"clip.mp4"}`, ToolImages: []images.Attachment{{Path: "clip.mp4", MIMEType: "image/png", Data: data, PresentationOnly: true}}},
		{Role: "user", Content: "continue"},
	}
	wire, err := json.Marshal(llm.MessageParams("", messages, nil))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wire, []byte("data:image/png;base64,")) {
		t.Fatal("poster became an OpenAI input")
	}
	history, err := llm.CursorAgentHistoryForTest(messages, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range history {
		if bytes.Contains(item, []byte(base64.StdEncoding.EncodeToString(data))) {
			t.Fatal("poster became a Cursor input")
		}
	}
}

func TestReadFileGIFUsesSingleFrameVisionSnapshot(t *testing.T) {
	var data bytes.Buffer
	im := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	im.SetColorIndex(0, 0, 1)
	if err := gif.Encode(&data, im, nil); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "image.gif")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"path": path, "intent": "inspect GIF"})
	result, err := execDeferredToolForTest(&agenttools.Env{ProjRoot: root}, tooling.Invocation{Name: "readFile", Args: args})
	if err != nil {
		t.Fatal(err)
	}
	_, a := images.TakeToolAttachments(result)
	if len(a) != 1 || a[0].MIMEType != "image/png" || a[0].PresentationOnly || !bytes.HasPrefix(a[0].Data, []byte("\x89PNG")) {
		t.Fatal("GIF must retain original path and attach one PNG frame")
	}
	if a[0].Path != path {
		t.Fatal("original GIF path lost")
	}
}
