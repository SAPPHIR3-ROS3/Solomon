package test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
)

func TestReadBigMediaOrchestrate(t *testing.T) {
	const path = "large-image"
	img := image.NewNRGBA(image.Rect(0, 0, 1600, 1200))
	random := rand.New(rand.NewSource(1))
	if _, err := random.Read(img.Pix); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	original := buffer.Bytes()
	if len(original) <= 5<<20 {
		t.Fatal("fixture must exceed 5 MiB")
	}

	source := `package main
import "sdk"
func main() {
 r, err := sdk.ReadBigMedia("large-image", "inspect local image")
 if err != nil { panic(err) }
 if r.Type != "image" || r.MIMEType != "image/png" || r.SizeBytes > 5<<20 || r.Width >= 1600 { panic("invalid reduced image metadata") }
}`
	args, err := json.Marshal(map[string]string{"source": source, "intent": "inspect image"})
	if err != nil {
		t.Fatal(err)
	}
	backend := &turnScriptBackend{protocol: llm.ProtocolOpenAI, turns: []llm.AssistantTurnResult{
		{ToolCalls: []llm.AssistantToolCall{{ID: "big-media", Name: "orchestrate", Arguments: string(args)}}},
		{Content: "image received"},
	}}
	rt := newTurnLoopRuntime(t, backend, nil, func(r *agentruntime.Runtime) {
		if err := os.WriteFile(filepath.Join(r.ProjRoot, path), original, 0600); err != nil {
			t.Fatal(err)
		}
	})
	if err := rt.RunAgentTurnsForTest(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, message := range rt.Session.Messages {
		if message.Role != "tool" || message.ToolCallID != "big-media" {
			continue
		}
		if len(message.ToolImages) != 1 {
			t.Fatalf("missing attachment: %s", message.Content)
		}
		preview := message.ToolImages[0]
		if preview.MIMEType != "image/png" || len(preview.Data) > 5<<20 {
			t.Fatalf("invalid preview: %s, %d bytes", preview.MIMEType, len(preview.Data))
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(preview.Data))
		if err != nil || cfg.Width >= 1600 || cfg.Width*3 != cfg.Height*4 {
			t.Fatalf("invalid resized image: %+v, %v", cfg, err)
		}
		current, err := os.ReadFile(filepath.Join(rt.ProjRoot, path))
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("source image changed or unavailable: %v", err)
		}
		return
	}
	t.Fatal("orchestrate tool result missing")
}
