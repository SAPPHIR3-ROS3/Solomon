package clipboard

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ReadImage reads image bytes without changing the clipboard.
func ReadImage(ctx context.Context) ([]byte, error) {
	if runtime.GOOS == "linux" {
		for _, source := range []struct {
			name string
			list []string
			read func(string) []string
		}{
			{"wl-paste", []string{"--list-types"}, func(mime string) []string { return []string{"--no-newline", "--type", mime} }},
			{"xclip", []string{"-selection", "clipboard", "-t", "TARGETS", "-o"}, func(mime string) []string { return []string{"-selection", "clipboard", "-t", mime, "-o"} }},
		} {
			types, err := exec.CommandContext(ctx, source.name, source.list...).Output()
			if err != nil {
				continue
			}
			for _, mime := range []string{"image/png", "image/jpeg", "image/webp", "image/gif"} {
				for _, offered := range strings.Fields(string(types)) {
					if offered != mime {
						continue
					}
					data, err := exec.CommandContext(ctx, source.name, source.read(mime)...).Output()
					if err == nil && len(data) > 0 {
						return data, nil
					}
				}
			}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrNoImage
	}
	if !HasImage() {
		return nil, ErrNoImage
	}
	dir, err := os.MkdirTemp("", "solomon-clipboard-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path, err := PasteImage(dir, "paste", 0)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
