//go:build linux

package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	imagepkg "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
)

func TestNativeX11Clipboard(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_X11_CLIPBOARD") != "1" {
		t.Skip("requires an isolated X11 display")
	}
	peer, err := exec.LookPath("xclip")
	if err != nil {
		t.Skip("xclip unavailable as a test peer")
	}
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jcWQAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	var bigPNG, jpegData bytes.Buffer
	large := imagepkg.NewRGBA(imagepkg.Rect(0, 0, 600, 600))
	random := rand.New(rand.NewSource(1))
	if _, err := random.Read(large.Pix); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&bigPNG, large); err != nil {
		t.Fatal(err)
	}
	picture := imagepkg.NewRGBA(imagepkg.Rect(0, 0, 2, 3))
	picture.Set(0, 0, color.White)
	if err := jpeg.Encode(&jpegData, picture, nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("PATH", t.TempDir())
	for _, fixture := range []struct {
		mime string
		data []byte
	}{
		{"image/png", data}, {"image/png", bigPNG.Bytes()}, {"image/jpeg", jpegData.Bytes()},
	} {
		command := exec.Command(peer, "-selection", "clipboard", "-t", fixture.mime, "-i")
		command.Stdin = bytes.NewReader(fixture.data)
		if err := command.Run(); err != nil {
			t.Fatal(err)
		}
		if !clipboard.HasImage() {
			t.Fatalf("native X11 probe did not detect %s (%d bytes)", fixture.mime, len(fixture.data))
		}
		actual, err := clipboard.ReadImage(context.Background())
		if err != nil || !bytes.Equal(actual, fixture.data) {
			t.Fatalf("native X11 read returned %d bytes: %v", len(actual), err)
		}
		path, err := clipboard.PasteImage(t.TempDir(), "image", 1)
		if err != nil {
			t.Fatal(err)
		}
		actual, err = os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatal(err)
		}
		if fixture.mime == "image/png" && !bytes.Equal(actual, fixture.data) {
			t.Fatal("native X11 paste modified the PNG")
		}
		if _, err := png.DecodeConfig(bytes.NewReader(actual)); err != nil {
			t.Fatalf("native X11 paste did not save PNG: %v", err)
		}
	}
	socket := filepath.Join(t.TempDir(), "wayland-no-data-control")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("WAYLAND_DISPLAY", socket)
	waylandDone := make(chan error, 1)
	go func() {
		conn, err := listener.AcceptUnix()
		if err == nil {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err = io.CopyN(io.Discard, conn, 24); err == nil {
				_, err = conn.Write(waylandTestPayload(3, 12<<16, 0))
			}
		}
		waylandDone <- err
	}()
	actual, err := clipboard.ReadImage(context.Background())
	if err != nil || !bytes.Equal(actual, jpegData.Bytes()) {
		t.Fatalf("clipboard did not fall back from Wayland to X11: %d bytes, %v", len(actual), err)
	}
	if err := <-waylandDone; err != nil {
		t.Fatal(err)
	}

	for _, fixture := range []struct {
		mime, content, hint string
	}{
		{"text/plain", "ordinary text", "copy the image itself"},
		{"image/png", "broken image", "export it as PNG"},
	} {
		t.Setenv("WAYLAND_DISPLAY", "")
		command := exec.Command(peer, "-selection", "clipboard", "-t", fixture.mime, "-i")
		command.Stdin = strings.NewReader(fixture.content)
		if err := command.Run(); err != nil {
			t.Fatal(err)
		}
		if _, err := clipboard.PasteImage(t.TempDir(), "image", 1); err == nil || !strings.Contains(err.Error(), fixture.hint) {
			t.Fatalf("expected recovery hint %q for %s, got %v", fixture.hint, fixture.mime, err)
		}
	}

}

func TestNativeClipboardWithoutDisplay(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("PATH", t.TempDir())
	if _, err := clipboard.ReadImage(context.Background()); !errors.Is(err, clipboard.ErrUnavailable) || !strings.Contains(err.Error(), "desktop session") {
		t.Fatalf("expected clipboard unavailable: %v", err)
	}
}

func TestNativeClipboardWaylandFailureMessages(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response []byte
		want     error
		hint     string
	}{
		{"no data control", waylandTestPayload(3, 12<<16, 0), clipboard.ErrUnavailable, "Wayland data-control"},
		{"timeout", nil, context.DeadlineExceeded, "Keep the source app open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			socket := filepath.Join(t.TempDir(), "wayland-error")
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			t.Setenv("WAYLAND_DISPLAY", socket)
			t.Setenv("DISPLAY", "")
			t.Setenv("PATH", t.TempDir())
			done := make(chan struct{})
			defer close(done)
			go func() {
				conn, err := listener.AcceptUnix()
				if err != nil {
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(time.Second))
				if _, err := io.CopyN(io.Discard, conn, 24); err == nil && len(tc.response) != 0 {
					conn.Write(tc.response)
				}
				<-done
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			if _, err := clipboard.ReadImage(ctx); !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("expected %v with hint %q, got %v", tc.want, tc.hint, err)
			}
		})
	}
}

func TestNativeWaylandClipboard(t *testing.T) {
	for _, tc := range []struct {
		name, manager, mime string
	}{
		{"ext-data-control", "ext_data_control_manager_v1", "image/png"},
		{"wlr-data-control", "zwlr_data_control_manager_v1", "image/jpeg"},
		{"empty-selection", "ext_data_control_manager_v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "wayland-test")
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: file, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			t.Setenv("WAYLAND_DISPLAY", file)
			t.Setenv("DISPLAY", "")
			t.Setenv("PATH", dir)
			var image bytes.Buffer
			picture := imagepkg.NewRGBA(imagepkg.Rect(0, 0, 2, 3))
			switch tc.mime {
			case "image/png":
				if err := png.Encode(&image, picture); err != nil {
					t.Fatal(err)
				}
			case "image/jpeg":
				if err := jpeg.Encode(&image, picture, nil); err != nil {
					t.Fatal(err)
				}
			}
			result := make(chan error, 1)
			go func() {
				conn, err := listener.AcceptUnix()
				if err != nil {
					result <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				result <- serveWaylandClipboard(conn, tc.manager, tc.mime, image.Bytes())
			}()
			actual, err := clipboard.ReadImage(context.Background())
			if tc.mime == "" {
				if !errors.Is(err, clipboard.ErrNoImage) {
					t.Fatalf("empty selection should report no image, got %v", err)
				}
			} else if err != nil || !bytes.Equal(actual, image.Bytes()) {
				t.Fatalf("native Wayland read returned %d bytes: %v", len(actual), err)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func waylandTestPayload(values ...uint32) []byte {
	var result []byte
	for _, v := range values {
		result = binary.NativeEndian.AppendUint32(result, v)
	}
	return result
}

func waylandTestText(value string) []byte {
	data := make([]byte, 4+((len(value)+4)&^3))
	binary.NativeEndian.PutUint32(data, uint32(len(value)+1))
	copy(data[4:], value)
	return data
}

func serveWaylandClipboard(conn *net.UnixConn, manager, mime string, image []byte) error {
	read := func(object, opcode uint32) error {
		var header [8]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return err
		}
		id, flags := binary.NativeEndian.Uint32(header[:]), binary.NativeEndian.Uint32(header[4:])
		if id != object || flags&0xffff != opcode || flags>>16 < 8 {
			return fmt.Errorf("unexpected Wayland request %d/%d", id, flags&0xffff)
		}
		_, err := io.CopyN(io.Discard, conn, int64(flags>>16)-8)
		return err
	}
	write := func(id, opcode uint32, payload []byte) error {
		data := append(waylandTestPayload(id, uint32(8+len(payload))<<16|opcode), payload...)
		_, err := conn.Write(data)
		return err
	}
	for _, req := range [][2]uint32{{1, 1}, {1, 0}} {
		if err := read(req[0], req[1]); err != nil {
			return err
		}
	}
	for i, name := range []string{manager, "wl_seat"} {
		global := append(waylandTestPayload(uint32(i+10)), waylandTestText(name)...)
		if err := write(2, 0, append(global, waylandTestPayload(1)...)); err != nil {
			return err
		}
	}
	if err := write(3, 0, waylandTestPayload(0)); err != nil {
		return err
	}
	for _, req := range [][2]uint32{{2, 0}, {2, 0}, {4, 1}, {1, 0}} {
		if err := read(req[0], req[1]); err != nil {
			return err
		}
	}
	if len(image) == 0 {
		if err := write(6, 1, waylandTestPayload(0)); err != nil {
			return err
		}
		return write(7, 0, waylandTestPayload(0))
	}
	const offer = 0xff000000
	for _, event := range []struct {
		id, opcode uint32
		payload    []byte
	}{
		{6, 0, waylandTestPayload(offer)},
		{offer, 0, waylandTestText("text/plain")},
		{offer, 0, waylandTestText(mime)},
		{6, 1, waylandTestPayload(offer)},
		{7, 0, waylandTestPayload(0)},
	} {
		if err := write(event.id, event.opcode, event.payload); err != nil {
			return err
		}
	}
	wire, control := make([]byte, 128), make([]byte, unix.CmsgSpace(4))
	n, nc, _, _, err := conn.ReadMsgUnix(wire, control)
	if err != nil {
		return err
	}
	expected := waylandTestText(mime)
	if n != 8+len(expected) || binary.NativeEndian.Uint32(wire) != offer ||
		binary.NativeEndian.Uint32(wire[4:]) != uint32(n)<<16 || !bytes.Equal(wire[8:n], expected) {
		return fmt.Errorf("expected Wayland offer receive request")
	}
	messages, err := unix.ParseSocketControlMessage(control[:nc])
	if err != nil || len(messages) != 1 {
		return fmt.Errorf("expected one Wayland ancillary message: %v", err)
	}
	rights, err := unix.ParseUnixRights(&messages[0])
	if err != nil || len(rights) != 1 {
		return fmt.Errorf("expected one Wayland file descriptor: %v", err)
	}
	pipe := os.NewFile(uintptr(rights[0]), "clipboard test")
	defer pipe.Close()
	_, err = pipe.Write(image)
	return err
}
