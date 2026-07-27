package btw

import "testing"

type muxTestWriter struct {
	fd uintptr
}

func (w muxTestWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w muxTestWriter) Fd() uintptr                 { return w.fd }

func TestOutputMuxTerminalFD(t *testing.T) {
	const want = uintptr(42)
	mux := NewOutputMux(muxTestWriter{fd: want})

	got, ok := mux.TerminalFD()
	if !ok || got != want {
		t.Fatalf("TerminalFD() = (%d, %t), want (%d, true)", got, ok, want)
	}
}

func TestOutputMuxTerminalFDWithoutDescriptor(t *testing.T) {
	mux := NewOutputMux(discardMuxTestWriter{})

	if got, ok := mux.TerminalFD(); ok || got != 0 {
		t.Fatalf("TerminalFD() = (%d, %t), want (0, false)", got, ok)
	}
}

type discardMuxTestWriter struct{}

func (discardMuxTestWriter) Write(p []byte) (int, error) { return len(p), nil }
