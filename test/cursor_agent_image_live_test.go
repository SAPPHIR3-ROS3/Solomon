package test

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

func TestCursorAgentImageLive(t *testing.T) {
	if os.Getenv("SOLOMON_CURSOR_DIRECT_LIVE") != "1" {
		t.Skip("live subscription test")
	}
	file, err := os.CreateTemp(t.TempDir(), "cursor-red-*.png")
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			picture.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	digest, err := images.DigestFromFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := config.ProviderByName(cfg, config.ProviderNameCursorSub)
	if p == nil {
		t.Fatal("Cursor Sub provider missing")
	}
	backend, err := llm.NewCompletionBackend(context.Background(), cfg, p)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	message := "What is the color of this image? Reply with only the color name in English. " + images.PlaceholderStored(1, digest)
	result, err := backend.StreamTurn(ctx, llm.TurnRequest{Model: "cursor-grok-4.6-high", Messages: []chatstore.Message{{Role: "user", Content: message}}, ImageFiles: map[int]string{1: file.Name()}}, io.Discard, llm.StreamOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(result.Content), "red") {
		t.Fatalf("image was not understood: %q", result.Content)
	}
}
