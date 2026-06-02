package test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
)

func tinyPNGPath(t *testing.T) string {
	t.Helper()
	tinyPNG, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAusBWFYpXQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ok.png")
	if err := os.WriteFile(path, tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImageToken_wireInvisibleDigestVisibleLabel(t *testing.T) {
	path := tinyPNGPath(t)
	digest, err := images.DigestFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wire := images.PlaceholderWithDigest(0, digest)
	if wire == images.VisibleTag(0) {
		t.Fatal("wire token must include invisible digest")
	}
	if !strings.Contains(wire, "\u200b") {
		t.Fatalf("missing SEP: %q", wire)
	}
	if strings.Contains(wire, "abcdef0123456789") {
		t.Fatal("sha must not appear as visible hex")
	}
	display := images.ExpandForDisplay(wire)
	if display != "[img-0]" {
		t.Fatalf("display want [img-0], got %q", display)
	}
}

func TestImageToken_roundTripCanonicalize(t *testing.T) {
	path := tinyPNGPath(t)
	digest, err := images.DigestFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wire := images.PlaceholderWithDigest(0, digest)
	stored := images.CanonicalizeUserLineForStorage(wire, nil)
	if stored != wire {
		t.Fatalf("canonicalize idempotent\ngot %q\nwant %q", stored, wire)
	}
}

func TestImageToken_legacyNoAttach(t *testing.T) {
	parts := llm.BuildUserContentParts("[img-0]", nil)
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatal("expected text only for legacy tag")
	}
	if parts[0].OfImageURL != nil {
		t.Fatal("bare tag without image file must not attach")
	}
	if got := parts[0].GetText(); got != nil && strings.Contains(*got, "[img-") {
		t.Fatalf("bare tag must not reach API: %q", *got)
	}
}

func TestImageToken_validStoredAttach(t *testing.T) {
	path := tinyPNGPath(t)
	digest, err := images.DigestFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tok := images.PlaceholderStored(0, digest)
	parts := llm.BuildUserContentParts(tok, map[int]string{0: path})
	if len(parts) != 1 || parts[0].OfImageURL == nil {
		t.Fatalf("want image part, got %+v", parts)
	}
}

func TestImageToken_assistantIgnored(t *testing.T) {
	path := tinyPNGPath(t)
	digest, _ := images.DigestFromFile(path)
	tok := images.PlaceholderStored(0, digest)
	msgs := []chatstore.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: tok},
	}
	params := llm.MessageParams("", msgs, map[int]string{0: path})
	if len(params) < 3 {
		t.Fatalf("params len %d", len(params))
	}
	ap := params[2].OfAssistant
	if ap == nil {
		t.Fatal("expected assistant")
	}
	if strings.Contains(ap.Content.OfString.Value, "\u200b") {
		t.Fatal("assistant must not pass PUA through scrubbed content")
	}
}

func TestImageToken_repairStripsLegacyLiteral(t *testing.T) {
	s := &chatstore.Session{
		Messages: []chatstore.Message{{Role: "user", Content: "see [img-1] in docs"}},
	}
	chatstore.RepairSessionMalformedImages(s)
	if strings.Contains(s.Messages[0].Content, "[img-") {
		t.Fatalf("repair should strip legacy literal: %q", s.Messages[0].Content)
	}
}

func TestImageToken_repairMigratesLegacyWithFile(t *testing.T) {
	path := tinyPNGPath(t)
	s := &chatstore.Session{
		Messages:   []chatstore.Message{{Role: "user", Content: "[img-0]"}},
		ImageFiles: map[int]string{0: path},
	}
	chatstore.RepairSessionMalformedImages(s)
	if !strings.Contains(s.Messages[0].Content, "\u200b") {
		t.Fatalf("want migrated token: %q", s.Messages[0].Content)
	}
}

func TestNormalizeREPLBuffer_collapsesWire(t *testing.T) {
	path := tinyPNGPath(t)
	digest, _ := images.DigestFromFile(path)
	wire := []rune(images.PlaceholderWithDigest(0, digest))
	line, pos := images.NormalizeREPLBuffer(wire, len(wire))
	if string(line) != images.VisibleTag(0) {
		t.Fatalf("normalize got %q", string(line))
	}
	if pos != len(line) {
		t.Fatalf("cursor pos %d want %d", pos, len(line))
	}
}

func TestImgTagRuneBounds_jumpAndDelete(t *testing.T) {
	tag := []rune(images.VisibleTag(1))
	prefix := []rune("see ")
	line := append(append(prefix, tag...), []rune(" ok")...)
	tagStart := len(prefix)
	tagEnd := tagStart + len(tag)

	if got := llm.JumpLeftOverImgTag(line, tagEnd); got != tagStart {
		t.Fatalf("jump left from after tag: got %d want %d", got, tagStart)
	}
	if got := llm.JumpRightOverImgTag(line, tagStart); got != tagEnd {
		t.Fatalf("jump right from before tag: got %d want %d", got, tagEnd)
	}
	mid := tagStart + len(tag)/2
	if got := llm.JumpLeftOverImgTag(line, mid); got != tagStart {
		t.Fatalf("jump left from inside: got %d want %d", got, tagStart)
	}

	newLine, newPos, ok := llm.BackspaceOverImgTag(line, tagEnd)
	if !ok || newPos != tagStart {
		t.Fatalf("backspace over tag: ok=%v pos=%d", ok, newPos)
	}
	if string(newLine) != "see  ok" {
		t.Fatalf("backspace result %q", string(newLine))
	}
}

func TestBackspaceImgFragment_partialWire(t *testing.T) {
	path := tinyPNGPath(t)
	digest, _ := images.DigestFromFile(path)
	wire := []rune(images.PlaceholderWithDigest(0, digest))
	line := append(wire[:len(wire)-1], []rune(" x")...)
	pos := len(wire) - 1
	newLine, newPos, ok := images.BackspaceImgFragment(line, pos)
	if !ok {
		t.Fatal("expected fragment backspace")
	}
	if strings.Contains(string(newLine), "\u200b") || strings.Contains(string(newLine), "[img-") {
		t.Fatalf("fragment should remove tag: %q", string(newLine))
	}
	if newPos != 0 {
		t.Fatalf("pos %d want 0", newPos)
	}
}

func TestMaskPUAPayloadForDisplay_sameRuneLen(t *testing.T) {
	path := tinyPNGPath(t)
	digest, _ := images.DigestFromFile(path)
	line := []rune(images.PlaceholderWithDigest(0, digest))
	masked := images.MaskPUAPayloadForDisplay(line)
	if len(masked) != len(line) {
		t.Fatalf("mask must preserve length for readline cursor")
	}
	for i, r := range line {
		if r >= 0xE000 && r <= 0xE0FF && masked[i] != images.PayloadSep {
			t.Fatalf("PUA at %d not masked", i)
		}
	}
}

func TestImageToken_hashMismatchStrippedOnRepair(t *testing.T) {
	path := tinyPNGPath(t)
	var bad [32]byte
	bad[0] = 0xff
	tok := images.PlaceholderStored(0, bad)
	s := &chatstore.Session{
		Messages:   []chatstore.Message{{Role: "user", Content: tok}},
		ImageFiles: map[int]string{0: path},
	}
	chatstore.RepairSessionMalformedImages(s)
	if strings.Contains(s.Messages[0].Content, "\u200b") {
		t.Fatalf("mismatch token should be stripped: %q", s.Messages[0].Content)
	}
}
