// Package terminalmedia adapts image previews to human-facing terminal output.
// Rendered characters are never persisted or included in model/tool payloads.
package terminalmedia

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/imagepreview"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/mediaartifact"
	"golang.org/x/term"
)

func terminalFD(w io.Writer) (uintptr, bool) {
	if f, ok := w.(interface{ TerminalFD() (uintptr, bool) }); ok {
		return f.TerminalFD()
	}
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		return f.Fd(), true
	}
	return 0, false
}

// Options honors the actual output descriptor, including Solomon's output mux.
// It never probes stdin or writes terminal capability requests into a chat.
func Options(w io.Writer) imagepreview.Options {
	o := imagepreview.Options{Width: 80, Height: 40, Colors: imagepreview.None, Background: color.RGBA{A: 255}}
	fd, ok := terminalFD(w)
	tty := ok && term.IsTerminal(int(fd))
	if tty {
		if width, height, err := term.GetSize(int(fd)); err == nil {
			o.Width = max(1, min(120, width-2))
			o.Height = max(1, min(40, height-4))
		}
	}
	o.Colors = imagepreview.DetectColors(tty)

	if c := os.Getenv("SOLOMON_IMAGE_COLOR"); c != "" && c != "auto" {
		o.Colors = imagepreview.Colors(c)
	}
	// Automatic previews in captured logs never carry ANSI, even with an override.
	if !tty || os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" {
		o.Colors = imagepreview.None
	}
	return o
}
func enabled(w io.Writer) bool {
	if os.Getenv("SOLOMON_IMAGE_PREVIEW") == "off" || os.Getenv("TERM") == "dumb" {
		return false
	}
	fd, ok := terminalFD(w)
	return ok && term.IsTerminal(int(fd))
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}

func writePreview(ctx context.Context, w io.Writer, path, kind string, load func() (image.Image, error)) {
	// The original path remains visible even if decoding fails or previews are off.
	label := "Image"
	if kind == "video" {
		label = "Video"
	} else if kind == "gif" {
		label = "GIF"
	}
	defer fmt.Fprintf(w, "  %s: %s\n", label, safe(path))
	if !enabled(w) || ctx.Err() != nil {
		return
	}
	img, err := load()
	if err == nil {
		err = imagepreview.Write(ctx, w, img, Options(w))
	}
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(w, "  Preview unavailable: %s\n", safe(err.Error()))
	}
}

// WriteAttachments renders saved snapshots, so replay survives source edits or
// deletion. Return their paths for deduplicating ordinary result metadata.
func WriteAttachments(ctx context.Context, w io.Writer, attachments []images.Attachment) map[string]bool {
	seen := map[string]bool{}
	for _, a := range attachments {
		if ctx.Err() != nil {
			break
		}
		key := mediaartifact.LocalPath(a.Path, "")
		if seen[key] {
			continue
		}
		seen[key] = true
		writePreview(ctx, w, a.Path, a.MediaType, func() (image.Image, error) { return imagepreview.Decode(bytes.NewReader(a.Data)) })
	}
	return seen
}

// WriteResult recognizes explicit local media artifacts, never arbitrary text
// or base64. Video producers can provide preview_path (or previewPath) rather
// than introducing a video decoder into Solomon. No external command is run.
func WriteResult(ctx context.Context, w io.Writer, result any, root string, seen map[string]bool) {
	if seen == nil {
		seen = map[string]bool{}
	}
	for _, d := range mediaartifact.Find(result, root) {
		if ctx.Err() != nil {
			return
		}
		if seen[d.Path] {
			continue
		}
		seen[d.Path] = true
		if d.PreviewPath == "" {
			fmt.Fprintf(w, "  Media: %s (no preview supplied)\n", safe(d.Path))
			continue
		}
		writePreview(ctx, w, d.Path, d.Kind, func() (image.Image, error) { return imagepreview.Open(d.PreviewPath) })
	}
}

// WriteReferences emits only presentation output. Saved snapshots can be
// supplied in seen, so replay never rereads files that have a stored preview.
func WriteReferences(ctx context.Context, w io.Writer, content, root string, skip ...map[string]bool) {
	if !enabled(w) {
		return
	}
	seen := map[string]bool{}
	if len(skip) > 0 && skip[0] != nil {
		seen = skip[0]
	}
	for _, d := range mediaartifact.References(content, root) {
		if ctx.Err() != nil {
			return
		}
		if seen[d.Path] {
			continue
		}
		seen[d.Path] = true
		if d.PreviewPath == "" {
			fmt.Fprintf(w, "  Media: %s (no preview supplied)\n", safe(d.Path))
			continue
		}
		writePreview(ctx, w, d.Path, d.Kind, func() (image.Image, error) { return imagepreview.Open(d.PreviewPath) })
	}
}
func localPath(raw, root string) string { return mediaartifact.LocalPath(raw, root) }
