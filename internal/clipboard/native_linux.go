package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"net"
	"os"
	"time"

	_ "golang.org/x/image/webp"
)

const maxClipboardBytes = 32 << 20
const clipboardTimeout = 2 * time.Second

var clipboardImageMIMEs = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

func checkLinuxImageBackend() error {
	if os.Getenv("WAYLAND_DISPLAY") == "" && os.Getenv("DISPLAY") == "" && os.Getenv("XDG_RUNTIME_DIR") == "" {
		return fmt.Errorf("%w: no Wayland or X11 display. Run Solomon from a terminal in your desktop session or attach the image by file path", ErrUnavailable)
	}
	return nil
}

func readLinuxImage(ctx context.Context) ([]byte, error) {
	return linuxClipboardImage(ctx, false)
}

func linuxClipboardImage(ctx context.Context, probe bool) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, clipboardTimeout)
	defer cancel()
	if err := checkLinuxImageBackend(); err != nil {
		return nil, err
	}
	data, waylandErr := readWaylandImage(ctx, probe)
	if waylandErr == nil || errors.Is(waylandErr, ErrImageTooLarge) {
		return data, waylandErr
	}
	var xErr error
	if os.Getenv("DISPLAY") != "" && ctx.Err() == nil {
		data, xErr = readXImage(ctx, probe)
		if xErr == nil || errors.Is(xErr, ErrImageTooLarge) {
			return data, xErr
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, ctx.Err()
	}
	for _, err := range []error{waylandErr, xErr} {
		var timeout net.Error
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
			return nil, fmt.Errorf("clipboard image transfer timed out: %w. Keep the source app open, copy the image again, or attach it by file path", context.DeadlineExceeded)
		}
	}
	if errors.Is(xErr, ErrNoImage) || errors.Is(waylandErr, ErrNoImage) {
		return nil, ErrNoImage
	}
	return nil, fmt.Errorf("%w: %w. Check that your session exposes Wayland data-control or that DISPLAY and XAUTHORITY point to an accessible X11 or XWayland session. You can also attach the image by file path",
		ErrUnavailable, errors.Join(waylandErr, xErr))
}

func linuxImageStatus() error {
	_, err := linuxClipboardImage(context.Background(), true)
	return err
}

func pasteImageLinux(path string) error {
	data, err := readLinuxImage(context.Background())
	if err != nil {
		return err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid clipboard image: %w. Copy it again or export it as PNG", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > (32<<20)/cfg.Height {
		return ErrImageTooLarge
	}
	if format != "png" {
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("cannot decode clipboard image: %w. Copy it again or export it as PNG", err)
		}
		var pngData bytes.Buffer
		if err := png.Encode(&pngData, decoded); err != nil {
			return fmt.Errorf("cannot encode clipboard image as PNG: %w. Export it as PNG and attach the file", err)
		}
		data = pngData.Bytes()
	}
	if len(data) > maxClipboardBytes {
		return ErrImageTooLarge
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot save clipboard attachment: %w. Check directory permissions and available disk space", err)
	}
	return nil
}
