//go:build !windows

package terminalmedia

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/creack/pty"
)

// Exercise the same TerminalFD adapter as the live output mux, while capturing
// output deterministically without touching the user's terminal.
type terminalCapture struct {
	bytes.Buffer
	fd uintptr
}

func (w *terminalCapture) TerminalFD() (uintptr, bool) { return w.fd, true }
func TestPTYAttachmentSnapshotAndReferences(t *testing.T) {
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
	t.Setenv("SOLOMON_IMAGE_PREVIEW", "")
	t.Setenv("SOLOMON_IMAGE_COLOR", "auto")
	out := &terminalCapture{fd: slave.Fd()}
	data := pngFixture(t)
	WriteAttachments(context.Background(), out, []images.Attachment{{Path: "/deleted.png", Data: data}})
	if !strings.Contains(out.String(), "\x1b[38;2;") || strings.Contains(out.String(), "unavailable") {
		t.Fatal("snapshot preview failed", out.String())
	}
	path := filepath.Join(t.TempDir(), "fixture.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	WriteReferences(context.Background(), out, "```markdown\n![ignored](missing.png)\n```\n![fixture](<"+path+">)\n[again]("+path+")", "")
	if strings.Count(out.String(), "Image:") != 1 || strings.Contains(out.String(), "missing") {
		t.Fatal(out.String())
	}
	t.Setenv("SOLOMON_IMAGE_COLOR", "256")
	out.Reset()
	WriteAttachments(context.Background(), out, []images.Attachment{{Path: path, Data: data}})
	if !strings.Contains(out.String(), "38;5;") || strings.Contains(out.String(), "38;2;") {
		t.Fatal("256 override failed")
	}
	t.Setenv("SOLOMON_IMAGE_PREVIEW", "off")
	out.Reset()
	WriteAttachments(context.Background(), out, []images.Attachment{{Path: path, Data: data}})
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("off ignored")
	}
}
func TestPTYVideoUsesSuppliedPoster(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	t.Setenv("TERM", "xterm")
	t.Setenv("SOLOMON_IMAGE_PREVIEW", "")
	t.Setenv("SOLOMON_IMAGE_COLOR", "none")
	out := &terminalCapture{fd: slave.Fd()}
	dir := t.TempDir()
	poster := filepath.Join(dir, "poster.gif")
	var b bytes.Buffer
	im := image.NewPaletted(image.Rect(0, 0, 2, 4), color.Palette{color.Black, color.White})
	im.SetColorIndex(1, 1, 1)
	if err := gif.Encode(&b, im, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(poster, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	WriteResult(context.Background(), out, map[string]any{"type": "video", "path": "movie.mp4", "preview_path": "poster.gif"}, dir, nil)
	if !strings.Contains(out.String(), "movie.mp4") || strings.Contains(out.String(), "unavailable") || strings.Count(out.String(), "\n") < 2 {
		t.Fatal(out.String())
	}
}
