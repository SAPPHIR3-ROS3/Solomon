package clipboard

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type xClipboard struct {
	net.Conn
	window   uint32
	sequence uint16
}

func connectXClipboard(ctx context.Context) (*xClipboard, error) {
	display := os.Getenv("DISPLAY")
	host, part, ok := strings.Cut(display, ":")
	number, screenName, _ := strings.Cut(part, ".")
	index, err := strconv.Atoi(number)
	if !ok || err != nil || index < 0 || index > 59535 {
		return nil, ErrUnavailable
	}
	screen := 0
	if screenName != "" {
		screen, err = strconv.Atoi(screenName)
		if err != nil || screen < 0 {
			return nil, ErrUnavailable
		}
	}
	network, address := "unix", "/tmp/.X11-unix/X"+number
	if host != "" && host != "unix" && !strings.HasSuffix(host, "/unix") {
		network, address = "tcp", net.JoinHostPort(host, strconv.Itoa(6000+index))
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	client := &xClipboard{Conn: conn}
	if err := client.setup(xAuthority(number, host, network == "unix"), screen); err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

func xAuthority(display, host string, local bool) []byte {
	path := os.Getenv("XAUTHORITY")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		path = filepath.Join(home, ".Xauthority")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	reader := io.LimitReader(f, 1<<20)
	hostname, _ := os.Hostname()
	var wildcard []byte
	for {
		var family uint16
		if binary.Read(reader, binary.BigEndian, &family) != nil {
			return wildcard
		}
		fields := [4][]byte{}
		for i := range fields {
			var size uint16
			if binary.Read(reader, binary.BigEndian, &size) != nil {
				return wildcard
			}
			fields[i] = make([]byte, size)
			if _, err := io.ReadFull(reader, fields[i]); err != nil {
				return wildcard
			}
		}
		if (len(fields[1]) != 0 && string(fields[1]) != display) || string(fields[2]) != "MIT-MAGIC-COOKIE-1" {
			continue
		}
		addr := string(fields[0])
		if local && family == 256 && addr == hostname {
			return fields[3]
		}
		if !local && (family == 0 || family == 6) && net.IP(fields[0]).Equal(net.ParseIP(host)) {
			return fields[3]
		}
		if family == 65535 {
			wildcard = fields[3]
		}
	}
}

func (x *xClipboard) setup(cookie []byte, screen int) error {
	name := ""
	if len(cookie) != 0 {
		name = "MIT-MAGIC-COOKIE-1"
	}
	request := make([]byte, 12+aligned(len(name))+aligned(len(cookie)))
	request[0] = 'l'
	binary.LittleEndian.PutUint16(request[2:], 11)
	binary.LittleEndian.PutUint16(request[6:], uint16(len(name)))
	binary.LittleEndian.PutUint16(request[8:], uint16(len(cookie)))
	copy(request[12:], name)
	copy(request[12+aligned(len(name)):], cookie)
	if _, err := x.Write(request); err != nil {
		return err
	}
	header := make([]byte, 8)
	if _, err := io.ReadFull(x, header); err != nil {
		return err
	}
	body := make([]byte, 4*int(binary.LittleEndian.Uint16(header[6:])))
	if _, err := io.ReadFull(x, body); err != nil {
		return err
	}
	if header[0] != 1 || len(body) < 32 || screen >= int(body[20]) {
		return fmt.Errorf("X11 clipboard connection rejected")
	}
	x.window = binary.LittleEndian.Uint32(body[4:])
	offset := 32 + aligned(int(binary.LittleEndian.Uint16(body[16:]))) + 8*int(body[21])
	for i := 0; i <= screen; i++ {
		if offset+40 > len(body) {
			return fmt.Errorf("invalid X11 screen data")
		}
		if i == screen {
			break
		}
		depths := body[offset+39]
		offset += 40
		for range depths {
			if offset+8 > len(body) {
				return fmt.Errorf("invalid X11 depth data")
			}
			offset += 8 + 24*int(binary.LittleEndian.Uint16(body[offset+2:]))
		}
	}
	root := binary.LittleEndian.Uint32(body[offset:])
	create := make([]byte, 32)
	binary.LittleEndian.PutUint32(create, x.window)
	binary.LittleEndian.PutUint32(create[4:], root)
	binary.LittleEndian.PutUint16(create[12:], 1)
	binary.LittleEndian.PutUint16(create[14:], 1)
	binary.LittleEndian.PutUint16(create[18:], 1)
	binary.LittleEndian.PutUint32(create[24:], 1<<11)
	binary.LittleEndian.PutUint32(create[28:], 1<<22)
	_, err := x.request(1, 0, create)
	return err
}

func aligned(size int) int { return (size + 3) &^ 3 }

func (x *xClipboard) request(opcode, flag byte, payload []byte) (uint16, error) {
	wire := make([]byte, 4+aligned(len(payload)))
	wire[0], wire[1] = opcode, flag
	binary.LittleEndian.PutUint16(wire[2:], uint16(len(wire)/4))
	copy(wire[4:], payload)
	x.sequence++
	_, err := x.Write(wire)
	return x.sequence, err
}

func (x *xClipboard) packet() ([]byte, []byte, error) {
	packet := make([]byte, 32)
	if _, err := io.ReadFull(x, packet); err != nil {
		return nil, nil, err
	}
	switch packet[0] & 127 {
	case 0:
		return nil, nil, fmt.Errorf("X11 clipboard protocol error %d (request %d)", packet[1], packet[10])
	case 1, 35:
		size := uint64(binary.LittleEndian.Uint32(packet[4:])) * 4
		if size > maxClipboardBytes {
			return nil, nil, ErrImageTooLarge
		}
		extra := make([]byte, int(size))
		_, err := io.ReadFull(x, extra)
		return packet, extra, err
	default:
		return packet, nil, nil
	}
}

func (x *xClipboard) reply(sequence uint16) ([]byte, []byte, error) {
	for {
		packet, extra, err := x.packet()
		if err != nil || (packet[0] == 1 && binary.LittleEndian.Uint16(packet[2:]) == sequence) {
			return packet, extra, err
		}
	}
}

func (x *xClipboard) atom(name string, existing bool) (uint32, error) {
	body := make([]byte, 4+len(name))
	binary.LittleEndian.PutUint16(body, uint16(len(name)))
	copy(body[4:], name)
	flag := byte(0)
	if existing {
		flag = 1
	}
	sequence, err := x.request(16, flag, body)
	if err != nil {
		return 0, err
	}
	reply, _, err := x.reply(sequence)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(reply[8:]), nil
}

func words(values ...uint32) []byte {
	out := make([]byte, 4*len(values))
	for i, value := range values {
		binary.LittleEndian.PutUint32(out[i*4:], value)
	}
	return out
}

func (x *xClipboard) property(atom uint32) (uint32, byte, []byte, error) {
	sequence, err := x.request(20, 1, words(x.window, atom, 0, 0, maxClipboardBytes/4))
	if err != nil {
		return 0, 0, nil, err
	}
	reply, data, err := x.reply(sequence)
	if err != nil {
		return 0, 0, nil, err
	}
	format := reply[1]
	size := uint64(binary.LittleEndian.Uint32(reply[16:])) * uint64(format/8)
	if binary.LittleEndian.Uint32(reply[12:]) != 0 {
		return 0, 0, nil, ErrImageTooLarge
	}
	if size > uint64(len(data)) {
		return 0, 0, nil, fmt.Errorf("invalid X11 clipboard property")
	}
	return binary.LittleEndian.Uint32(reply[8:]), format, data[:int(size)], nil
}

func (x *xClipboard) read(selection, target, property, incremental, valueType uint32, valueFormat byte) ([]byte, error) {
	if _, err := x.request(24, 0, words(x.window, selection, target, property, 0)); err != nil {
		return nil, err
	}
	for {
		packet, _, err := x.packet()
		if err != nil {
			return nil, err
		}
		if packet[0]&127 != 31 || binary.LittleEndian.Uint32(packet[8:]) != x.window ||
			binary.LittleEndian.Uint32(packet[16:]) != target {
			continue
		}
		if binary.LittleEndian.Uint32(packet[20:]) == 0 {
			return nil, ErrNoImage
		}
		break
	}
	kind, format, data, err := x.property(property)
	if err != nil {
		return nil, err
	}
	if kind != incremental || incremental == 0 {
		if kind != valueType || format != valueFormat || len(data) == 0 {
			return nil, ErrNoImage
		}
		return data, nil
	}
	return x.readIncremental(valueType, valueFormat, property)
}

func (x *xClipboard) readIncremental(valueType uint32, valueFormat byte, property uint32) ([]byte, error) {
	var buffer bytes.Buffer
	for {
		packet, _, err := x.packet()
		if err != nil {
			return nil, err
		}
		if packet[0]&127 != 28 || packet[16] != 0 || binary.LittleEndian.Uint32(packet[4:]) != x.window ||
			binary.LittleEndian.Uint32(packet[8:]) != property {
			continue
		}
		kind, format, data, err := x.property(property)
		if err != nil {
			return nil, err
		}
		if buffer.Len()+len(data) > maxClipboardBytes {
			return nil, ErrImageTooLarge
		}
		if kind != valueType || format != valueFormat {
			return nil, fmt.Errorf("invalid incremental X11 clipboard image")
		}
		if len(data) == 0 {
			if buffer.Len() == 0 {
				return nil, ErrNoImage
			}
			return buffer.Bytes(), nil
		}
		buffer.Write(data)
	}
}

func readXImage(ctx context.Context, probe bool) ([]byte, error) {
	client, err := connectXClipboard(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()
	selection, err := client.atom("CLIPBOARD", true)
	if err != nil || selection == 0 {
		if err == nil {
			err = ErrNoImage
		}
		return nil, err
	}
	property, err := client.atom("SOLOMON_CLIPBOARD", false)
	if err != nil {
		return nil, err
	}
	incremental, err := client.atom("INCR", false)
	if err != nil {
		return nil, err
	}
	var advertised []byte
	if probe {
		targets, err := client.atom("TARGETS", false)
		if err != nil {
			return nil, err
		}
		if targets == 0 {
			return nil, ErrNoImage
		}
		const atomType = 4
		advertised, err = client.read(selection, targets, property, incremental, atomType, 32)
		if err != nil {
			return nil, err
		}
	}
	for _, mime := range clipboardImageMIMEs {
		target, err := client.atom(mime, true)
		if err != nil {
			return nil, err
		}
		if target == 0 {
			continue
		}
		if probe {
			for offset := 0; offset+4 <= len(advertised); offset += 4 {
				if binary.LittleEndian.Uint32(advertised[offset:]) == target {
					return nil, nil
				}
			}
			continue
		}
		data, err := client.read(selection, target, property, incremental, target, 8)
		if err != ErrNoImage {
			return data, err
		}
	}
	return nil, ErrNoImage
}
