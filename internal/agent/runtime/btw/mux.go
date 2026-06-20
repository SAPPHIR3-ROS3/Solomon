package btw

import (
	"bytes"
	"io"
	"sync"
)

type OutputMux struct {
	live io.Writer
	mu   sync.Mutex
	buf  bytes.Buffer
	mode bufferMode
}

type bufferMode int

const (
	modeLive bufferMode = iota
	modeBufferMain
)

func NewOutputMux(live io.Writer) *OutputMux {
	return &OutputMux{live: live, mode: modeLive}
}

func (m *OutputMux) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mode == modeBufferMain {
		return m.buf.Write(p)
	}
	return m.live.Write(p)
}

func (m *OutputMux) WriteLive(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.live.Write(p)
}

func (m *OutputMux) Live() io.Writer {
	return liveWriter{m}
}

type liveWriter struct{ m *OutputMux }

func (w liveWriter) Write(p []byte) (int, error) {
	return w.m.WriteLive(p)
}

func (m *OutputMux) SetBufferMain() {
	m.mu.Lock()
	m.mode = modeBufferMain
	m.mu.Unlock()
}

func (m *OutputMux) Buffering() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode == modeBufferMain
}

func (m *OutputMux) FlushBurst() {
	m.mu.Lock()
	if m.buf.Len() == 0 {
		m.mode = modeLive
		m.mu.Unlock()
		return
	}
	data := append([]byte(nil), m.buf.Bytes()...)
	m.buf.Reset()
	m.mode = modeLive
	w := m.live
	m.mu.Unlock()
	_, _ = w.Write(data)
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}
