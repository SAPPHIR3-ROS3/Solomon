package mediaartifact

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/imagepreview"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func previewBytes(data []byte) ([]byte, string, error) {
	if len(data) > imagepreview.MaxBytes {
		return nil, "", fmt.Errorf("media preview exceeds %d MiB", imagepreview.MaxBytes>>20)
	}
	mime, ok := images.MIMEForBinary(data)
	if !ok {
		return nil, "", fmt.Errorf("unsupported image preview")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("invalid preview: %w", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > imagepreview.MaxPixels/cfg.Height {
		return nil, "", fmt.Errorf("media preview exceeds pixel limit")
	}
	if mime == "image/gif" {
		// Keep the original GIF path, but snapshot one PNG frame for both terminal
		// replay and vision backends that do not accept animated image payloads.
		img, err := imagepreview.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			return nil, "", err
		}
		data = b.Bytes()
		mime = "image/png"
		if len(data) > imagepreview.MaxBytes {
			return nil, "", fmt.Errorf("decoded GIF preview exceeds limit")
		}
	}
	return data, mime, nil
}

func Snapshot(d Descriptor) (images.Attachment, error) {
	if d.PreviewPath == "" {
		return images.Attachment{}, fmt.Errorf("no poster supplied")
	}
	st, err := os.Stat(d.PreviewPath)
	if err != nil {
		return images.Attachment{}, err
	}
	if !st.Mode().IsRegular() || st.Size() > imagepreview.MaxBytes {
		return images.Attachment{}, fmt.Errorf("preview must be a regular file within the size limit")
	}
	f, err := os.Open(d.PreviewPath)
	if err != nil {
		return images.Attachment{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, imagepreview.MaxBytes+1))
	if err != nil {
		return images.Attachment{}, err
	}
	data, mime, err := previewBytes(data)
	if err != nil {
		return images.Attachment{}, err
	}
	return images.Attachment{Path: d.Path, PreviewPath: d.PreviewPath, MediaType: d.Kind, PresentationOnly: true, MIMEType: mime, Data: data}, nil
}

// Capture snapshots explicit tool artifacts before text truncation and before
// the WASM boundary discards unprinted results. Existing image snapshots win.
func Capture(result any, root string, existing []images.Attachment) []images.Attachment {
	out := append([]images.Attachment(nil), existing...)
	seen := map[string]bool{}
	total := 0
	for _, a := range existing {
		seen[LocalPath(a.Path, root)] = true
		total += len(a.Data)
	}
	for _, d := range Find(result, root) {
		if len(out) >= 8 || total >= imagepreview.MaxBytes {
			break
		}
		if seen[d.Path] || d.PreviewPath == "" {
			continue
		}
		a, err := Snapshot(d)
		if err != nil || total+len(a.Data) > imagepreview.MaxBytes {
			continue
		}
		out = append(out, a)
		seen[d.Path] = true
		total += len(a.Data)
	}
	return out
}

// Store preserves a binary MCP image as a content-addressed original. Textual
// tool results contain metadata only, never a base64 image masquerading as text.
// If preview decoding fails, the descriptor still points to the saved original.
func Store(data []byte, declaredMIME string) (Descriptor, images.Attachment, error) {
	var empty images.Attachment
	if len(data) == 0 || len(data) > imagepreview.MaxBytes {
		return Descriptor{}, empty, fmt.Errorf("invalid media payload size")
	}
	mime, ok := images.MIMEForBinary(data)
	if !ok {
		mime = strings.ToLower(declaredMIME)
	}
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/svg+xml": ".svg", "image/avif": ".avif"}[mime]
	if ext == "" {
		ext = ".bin"
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return Descriptor{}, empty, err
	}
	dir, err := filepath.Abs(filepath.Join(home, "media"))
	if err != nil {
		return Descriptor{}, empty, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Descriptor{}, empty, err
	}
	hash := sha256.Sum256(data)
	path := filepath.Join(dir, fmt.Sprintf("%x%s", hash, ext))
	if err := writeOriginal(path, data); err != nil {
		return Descriptor{}, empty, err
	}
	kind := "image"
	if mime == "image/gif" {
		kind = "gif"
	}
	d := Descriptor{kind, path, path}
	snapshot, snapshotMIME, err := previewBytes(data)
	if err != nil {
		return d, empty, err
	}
	// A native MCP image is also a vision input. Validate its pixel payload,
	// not only the header, so corrupt data cannot fail the next provider turn.
	// GIF conversion above has already decoded and validated its first frame.
	if mime != "image/gif" {
		if _, _, err := image.Decode(bytes.NewReader(snapshot)); err != nil {
			return d, empty, fmt.Errorf("invalid media pixels: %w", err)
		}
	}
	return d, images.Attachment{Path: path, PreviewPath: path, MediaType: kind, MIMEType: snapshotMIME, Data: snapshot}, nil
}
func writeOriginal(path string, data []byte) error {
	if st, err := os.Lstat(path); err == nil {
		if !st.Mode().IsRegular() {
			return fmt.Errorf("media cache target is not a regular file")
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, data) {
			return fmt.Errorf("media cache content mismatch")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".media-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// Rename publishes complete bytes atomically; concurrent identical artifacts
	// can safely replace one another under the same content-addressed filename.
	return os.Rename(f.Name(), path)
}

// CaptureReferences stores output previews separately from model-facing tool
// images; assistant links are presentation, not synthetic user image inputs.
func CaptureReferences(content, root string) []images.Attachment {
	var items []any
	for _, d := range References(content, root) {
		items = append(items, map[string]any{"type": d.Kind, "path": d.Path, "preview_path": d.PreviewPath})
	}
	return Capture(items, root, nil)
}
