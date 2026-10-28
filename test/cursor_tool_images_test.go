package test

import (
	"bytes"
	"testing"

	cursorauth "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/auth/cursor"
	"google.golang.org/protobuf/encoding/protowire"
)

// Decode the wire independently of the handwritten protobuf encoder.
func bytesFields(t *testing.T, wire []byte, field protowire.Number) [][]byte {
	t.Helper()
	var values [][]byte
	for len(wire) > 0 {
		num, typ, n := protowire.ConsumeTag(wire)
		if n < 0 {
			t.Fatal("invalid protobuf tag")
		}
		wire = wire[n:]
		if typ == protowire.BytesType {
			value, size := protowire.ConsumeBytes(wire)
			if size < 0 {
				t.Fatal("invalid protobuf bytes")
			}
			if num == field {
				values = append(values, value)
			}
			wire = wire[size:]
		} else {
			size := protowire.ConsumeFieldValue(num, typ, wire)
			if size < 0 {
				t.Fatal("invalid protobuf value")
			}
			wire = wire[size:]
		}
	}
	return values
}

func TestAgentToolResultImageWire(t *testing.T) {
	image := cursorauth.AgentImage{Data: []byte{1, 2, 3}, Mime: "image/webp"}
	wire := cursorauth.AgentToolResultMessageForTest(cursorauth.AgentToolCall{ExecNumber: 7, ExecID: "exec-7"}, "image read", false, []cursorauth.AgentImage{image})
	exec := bytesFields(t, wire, 2)[0]
	if got := bytesFields(t, exec, 15); len(got) != 1 || string(got[0]) != "exec-7" {
		t.Fatal("lost execution correlation")
	}
	result := bytesFields(t, exec, 11)[0]
	success := bytesFields(t, result, 1)[0]
	items := bytesFields(t, success, 1)
	if len(items) != 2 {
		t.Fatalf("content items = %d, want text and image", len(items))
	}
	text := bytesFields(t, items[0], 1)[0]
	if string(bytesFields(t, text, 1)[0]) != "image read" {
		t.Fatal("lost text result")
	}
	img := bytesFields(t, items[1], 2)[0]
	if !bytes.Equal(bytesFields(t, img, 1)[0], image.Data) || string(bytesFields(t, img, 2)[0]) != image.Mime {
		t.Fatal("incorrect MCP image bytes or MIME type")
	}
}
