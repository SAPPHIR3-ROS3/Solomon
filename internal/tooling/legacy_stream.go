package tooling

import (
	"encoding/json"
	"io"
	"strings"
)

type legacyStreamState int

const (
	streamOutside legacyStreamState = iota
	streamBuffering
	streamDone
)

type FormatToolLinesFunc func(name string, args json.RawMessage) []string

type LegacyStreamWriter struct {
	Out     io.Writer
	Format  FormatToolLinesFunc
	Allowed map[string]struct{}

	state           legacyStreamState
	held            []byte
	prefix          strings.Builder
	block           strings.Builder
	invs            []Invocation
	displayRendered bool
}

func NewLegacyStreamWriter(out io.Writer, format FormatToolLinesFunc, allowed map[string]struct{}) *LegacyStreamWriter {
	return &LegacyStreamWriter{Out: out, Format: format, Allowed: allowed}
}

func (w *LegacyStreamWriter) Invocations() []Invocation {
	return w.invs
}

func (w *LegacyStreamWriter) DisplayRendered() bool {
	return w.displayRendered
}

func (w *LegacyStreamWriter) TruncatedContent() string {
	if w.block.Len() == 0 {
		return w.prefix.String()
	}
	return w.prefix.String() + w.block.String()
}

func (w *LegacyStreamWriter) HasOpenToolCalls() bool {
	combined := w.prefix.String() + w.block.String() + string(w.held)
	lower := strings.ToLower(combined)
	if strings.Contains(lower, strings.ToLower(tagToolCallsOpen)) {
		return strings.Count(lower, strings.ToLower(tagToolCallsOpen)) > strings.Count(lower, strings.ToLower(tagToolCallsClose))
	}
	if strings.Contains(lower, "<tool_call>") {
		return strings.Count(lower, "<tool_call>") > strings.Count(lower, "</tool_call>")
	}
	return false
}

func (w *LegacyStreamWriter) Write(p []byte) (int, error) {
	if w.state == streamDone {
		return len(p), nil
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := len(p)
	data := append(w.held, p...)
	w.held = w.held[:0]
	for len(data) > 0 {
		var err error
		var rest []byte
		switch w.state {
		case streamOutside:
			rest, err = w.writeOutside(data)
		case streamBuffering:
			rest, err = w.writeBuffering(data)
		default:
			return n, nil
		}
		if err != nil {
			return n, err
		}
		data = rest
	}
	return n, nil
}

func (w *LegacyStreamWriter) Flush() error {
	if w.state == streamOutside && len(w.held) > 0 {
		_, err := w.Out.Write(w.held)
		w.prefix.Write(w.held)
		w.held = nil
		if err != nil {
			return err
		}
	}
	if f, ok := w.Out.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

func (w *LegacyStreamWriter) writeOutside(data []byte) ([]byte, error) {
	combined := string(data)
	idx := strings.Index(combined, tagToolCallsOpen)
	if idx < 0 {
		emit, hold := splitPartialTagSuffix(data, tagToolCallsOpen)
		if len(emit) > 0 {
			if _, err := w.Out.Write(emit); err != nil {
				return nil, err
			}
			w.prefix.Write(emit)
		}
		w.held = append(w.held[:0], hold...)
		return nil, nil
	}
	before := data[:idx]
	if len(before) > 0 {
		if _, err := w.Out.Write(before); err != nil {
			return nil, err
		}
		w.prefix.Write(before)
	}
	w.state = streamBuffering
	w.block.WriteString(tagToolCallsOpen)
	return data[idx+len(tagToolCallsOpen):], nil
}

func (w *LegacyStreamWriter) writeBuffering(data []byte) ([]byte, error) {
	w.block.Write(data)
	combined := w.block.String()
	idx := strings.Index(combined, tagToolCallsClose)
	if idx < 0 {
		holdLen := len(tagToolCallsClose) - 1
		if len(combined) >= holdLen {
			tail := combined[len(combined)-holdLen:]
			if strings.HasPrefix(tagToolCallsClose, tail) {
				w.block.Reset()
				w.block.WriteString(combined[:len(combined)-holdLen])
				w.held = append(w.held[:0], []byte(tail)...)
			}
		}
		return nil, nil
	}
	end := idx + len(tagToolCallsClose)
	fullBlock := combined[:end]
	w.block.Reset()
	w.block.WriteString(fullBlock)
	w.state = streamDone
	invs, err := ParseToolCallsBlock(normalizeLegacyToolBlock(fullBlock))
	if err != nil {
		return nil, err
	}
	if err := ValidateInvocationNames(invs, w.Allowed); err != nil {
		return nil, err
	}
	w.invs = invs
	if w.Format != nil {
		for _, inv := range invs {
			for _, line := range w.Format(inv.Name, inv.Args) {
				if _, err := io.WriteString(w.Out, line+"\n"); err != nil {
					return nil, err
				}
			}
		}
	}
	w.displayRendered = true
	return nil, ErrLegacyToolBlockComplete
}

func splitPartialTagSuffix(data []byte, tag string) (emit, hold []byte) {
	maxHold := len(tag) - 1
	if maxHold <= 0 || len(data) == 0 {
		return data, nil
	}
	for holdLen := maxHold; holdLen > 0; holdLen-- {
		if len(data) < holdLen {
			continue
		}
		suffix := string(data[len(data)-holdLen:])
		if strings.HasPrefix(tag, suffix) {
			return data[:len(data)-holdLen], data[len(data)-holdLen:]
		}
	}
	return data, nil
}
