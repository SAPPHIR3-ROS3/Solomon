package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNativeMCPImagesBecomeStoredAttachments(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	result, err := convertResult(&sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "generated"}, &sdkmcp.ImageContent{MIMEType: "image/png", Data: b.Bytes()}}})
	if err != nil {
		t.Fatal(err)
	}
	clean, attachments := images.TakeToolAttachments(result)
	if len(attachments) != 1 || !bytes.Equal(attachments[0].Data, b.Bytes()) {
		t.Fatal("image side channel missing")
	}
	if saved, e := os.ReadFile(attachments[0].Path); e != nil || !bytes.Equal(saved, b.Bytes()) {
		t.Fatal("original file missing")
	}
	payload, err := json.Marshal(clean)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte(base64.StdEncoding.EncodeToString(b.Bytes()))) || bytes.Contains(payload, []byte(images.ToolAttachmentsKey)) {
		t.Fatal("binary leaked into textual tool output")
	}
	if !bytes.Contains(payload, []byte("generated")) || !bytes.Contains(payload, []byte("path")) {
		t.Fatal("result metadata lost")
	}
}
func TestInvalidMCPImageDoesNotFailOtherContent(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	result, err := convertResult(&sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}, &sdkmcp.ImageContent{MIMEType: "image/png", Data: []byte("broken")}}})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result)
	if !bytes.Contains(data, []byte("preview_error")) || !bytes.Contains(data, []byte("ok")) {
		t.Fatal(string(data))
	}
}

func TestEmbeddedMCPImageBecomesAttachment(t *testing.T) {
	t.Setenv("SOLOMON_HOME", t.TempDir())
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	result, err := convertResult(&sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.EmbeddedResource{Resource: &sdkmcp.ResourceContents{URI: "memory:///preview", MIMEType: "image/png", Blob: b.Bytes()}}}})
	if err != nil {
		t.Fatal(err)
	}
	clean, a := images.TakeToolAttachments(result)
	if len(a) != 1 {
		t.Fatal("embedded image missing")
	}
	payload, _ := json.Marshal(clean)
	if bytes.Contains(payload, []byte("blob")) {
		t.Fatal("binary resource leaked into tool text")
	}
}
