package clipboard

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	waylandDisplay       = 1
	waylandRegistry      = 2
	waylandRegistryDone  = 3
	waylandManager       = 4
	waylandSeat          = 5
	waylandDevice        = 6
	waylandSelectionDone = 7
)

type waylandClipboard struct {
	*net.UnixConn
	offers   map[uint32][]string
	selected uint32
}

type waylandEvent struct {
	object uint32
	opcode uint16
	data   []byte
}

func readWaylandImage(ctx context.Context, probe bool) ([]byte, error) {
	path := os.Getenv("WAYLAND_DISPLAY")
	if path == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return nil, ErrUnavailable
		}
		path = "wayland-0"
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), path)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	client := &waylandClipboard{UnixConn: conn.(*net.UnixConn), offers: make(map[uint32][]string)}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		client.SetDeadline(deadline)
	}
	if err := client.openSelection(); err != nil {
		return nil, err
	}
	for _, mime := range clipboardImageMIMEs {
		for _, offered := range client.offers[client.selected] {
			if offered == mime {
				if probe {
					return nil, nil
				}
				return client.receive(ctx, mime)
			}
		}
	}
	return nil, ErrNoImage
}

func (c *waylandClipboard) openSelection() error {
	if err := c.send(waylandDisplay, 1, nativeWords(waylandRegistry), nil); err != nil {
		return err
	}
	if err := c.send(waylandDisplay, 0, nativeWords(waylandRegistryDone), nil); err != nil {
		return err
	}
	globals := make(map[string]uint32)
	for {
		event, err := c.event()
		if err != nil {
			return err
		}
		if event.object == waylandRegistryDone {
			break
		}
		if event.object == waylandRegistry && event.opcode == 0 && len(event.data) >= 8 {
			name, ok := waylandString(event.data[4:])
			if ok && globals[name] == 0 {
				globals[name] = binary.NativeEndian.Uint32(event.data)
			}
		}
	}
	manager := "ext_data_control_manager_v1"
	if globals[manager] == 0 {
		manager = "zwlr_data_control_manager_v1"
	}
	if globals[manager] == 0 || globals["wl_seat"] == 0 {
		return fmt.Errorf("%w: Wayland compositor has no clipboard data-control support", ErrUnavailable)
	}
	if err := c.bind(globals[manager], manager, waylandManager); err != nil {
		return err
	}
	if err := c.bind(globals["wl_seat"], "wl_seat", waylandSeat); err != nil {
		return err
	}
	if err := c.send(waylandManager, 1, nativeWords(waylandDevice, waylandSeat), nil); err != nil {
		return err
	}
	if err := c.send(waylandDisplay, 0, nativeWords(waylandSelectionDone), nil); err != nil {
		return err
	}
	for {
		event, err := c.event()
		if err != nil {
			return err
		}
		if event.object == waylandSelectionDone {
			return nil
		}
		if event.object == waylandDevice {
			switch event.opcode {
			case 0, 1:
				if len(event.data) < 4 {
					return fmt.Errorf("invalid Wayland clipboard event")
				}
				id := binary.NativeEndian.Uint32(event.data)
				if event.opcode == 0 {
					c.offers[id] = nil
				} else {
					c.selected = id
				}
			case 2:
				return ErrUnavailable
			}
		} else if formats, ok := c.offers[event.object]; ok && event.opcode == 0 {
			if mime, valid := waylandString(event.data); valid {
				c.offers[event.object] = append(formats, mime)
			}
		}
	}
}

func nativeWords(values ...uint32) []byte {
	out := make([]byte, 0, len(values)*4)
	for _, value := range values {
		out = binary.NativeEndian.AppendUint32(out, value)
	}
	return out
}

func waylandText(s string) []byte {
	buffer := make([]byte, 4+aligned(len(s)+1))
	binary.NativeEndian.PutUint32(buffer, uint32(len(s)+1))
	copy(buffer[4:], s)
	return buffer
}

func waylandString(data []byte) (string, bool) {
	if len(data) < 4 {
		return "", false
	}
	n := binary.NativeEndian.Uint32(data)
	if n == 0 || uint64(n) > uint64(len(data)-4) || data[3+int(n)] != 0 {
		return "", false
	}
	return string(data[4 : 3+int(n)]), true
}

func (c *waylandClipboard) bind(global uint32, name string, id uint32) error {
	payload := append(nativeWords(global), waylandText(name)...)
	payload = append(payload, nativeWords(1, id)...)
	return c.send(waylandRegistry, 0, payload, nil)
}

func (c *waylandClipboard) send(object uint32, opcode uint16, payload, rights []byte) error {
	message := append(nativeWords(object, uint32(8+len(payload))<<16|uint32(opcode)), payload...)
	n, _, err := c.WriteMsgUnix(message, rights, nil)
	if err == nil && n != len(message) {
		err = io.ErrShortWrite
	}
	return err
}

func (c *waylandClipboard) event() (waylandEvent, error) {
	var header [8]byte
	if _, err := io.ReadFull(c, header[:]); err != nil {
		return waylandEvent{}, err
	}
	object := binary.NativeEndian.Uint32(header[:])
	flags := binary.NativeEndian.Uint32(header[4:])
	size := int(flags >> 16)
	if size < 8 || size%4 != 0 {
		return waylandEvent{}, fmt.Errorf("invalid Wayland message size")
	}
	payload := make([]byte, size-8)
	if _, err := io.ReadFull(c, payload); err != nil {
		return waylandEvent{}, err
	}
	if object == waylandDisplay && uint16(flags) == 0 {
		return waylandEvent{}, fmt.Errorf("Wayland clipboard protocol error")
	}
	return waylandEvent{object: object, opcode: uint16(flags), data: payload}, nil
}

func (c *waylandClipboard) receive(ctx context.Context, mime string) ([]byte, error) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer readEnd.Close()
	stop := context.AfterFunc(ctx, func() { readEnd.Close() })
	defer stop()
	err = c.send(c.selected, 0, waylandText(mime), unix.UnixRights(int(writeEnd.Fd())))
	writeEnd.Close()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(readEnd, maxClipboardBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxClipboardBytes {
		return nil, ErrImageTooLarge
	}
	if len(data) == 0 {
		return nil, ErrNoImage
	}
	return data, nil
}
