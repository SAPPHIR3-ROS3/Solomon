package test

import (
	"bytes"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/btw"
)

func TestOutputMuxBufferAndFlushBurst(t *testing.T) {
	t.Parallel()
	var live bytes.Buffer
	mux := btw.NewOutputMux(&live)
	if _, err := mux.Write([]byte("live")); err != nil {
		t.Fatal(err)
	}
	if live.String() != "live" {
		t.Fatalf("live = %q", live.String())
	}
	mux.SetBufferMain()
	if _, err := mux.Write([]byte("buf")); err != nil {
		t.Fatal(err)
	}
	if live.String() != "live" {
		t.Fatalf("buffered write leaked: %q", live.String())
	}
	if _, err := mux.Live().Write([]byte("side")); err != nil {
		t.Fatal(err)
	}
	if live.String() != "liveside" {
		t.Fatalf("live writer = %q", live.String())
	}
	mux.FlushBurst()
	if live.String() != "livesidebuf" {
		t.Fatalf("after flush = %q", live.String())
	}
	if mux.Buffering() {
		t.Fatal("expected live mode after flush")
	}
}
