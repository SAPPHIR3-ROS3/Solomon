package cievents

import (
	"encoding/json"
	"io"
	"os"
)

type Sink interface {
	Emit(ev Event)
	StreamMode() bool
	Events() []Event
	FlushReport(meta ReportMeta, exitCode int, exitReason, finalContent string, usage any) error
}

type ReportMeta struct {
	Prompt    string
	Model     string
	Provider  string
	ProjHex   string
	Ephemeral bool
}

type RunReport struct {
	V            int     `json:"v"`
	Prompt       string  `json:"prompt"`
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	ProjHex      string  `json:"proj_hex"`
	Ephemeral    bool    `json:"ephemeral"`
	Events       []Event `json:"events"`
	ExitCode     int     `json:"exit_code"`
	ExitReason   string  `json:"exit_reason"`
	FinalContent string  `json:"final_content,omitempty"`
	Usage        any     `json:"usage,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type JSONLEmitter struct {
	out io.Writer
}

func NewJSONLEmitter(out io.Writer) *JSONLEmitter {
	if out == nil {
		out = os.Stdout
	}
	return &JSONLEmitter{out: out}
}

func (e *JSONLEmitter) Emit(ev Event) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = e.out.Write(b)
	if f, ok := e.out.(interface{ Sync() error }); ok {
		_ = f.Sync()
	}
}

func (e *JSONLEmitter) StreamMode() bool { return true }

func (e *JSONLEmitter) Events() []Event { return nil }

func (e *JSONLEmitter) FlushReport(ReportMeta, int, string, string, any) error {
	return nil
}

type JSONCollector struct {
	out    io.Writer
	events []Event
}

func NewJSONCollector(out io.Writer) *JSONCollector {
	if out == nil {
		out = os.Stdout
	}
	return &JSONCollector{out: out, events: make([]Event, 0, 64)}
}

func (c *JSONCollector) Emit(ev Event) {
	c.events = append(c.events, ev)
}

func (c *JSONCollector) StreamMode() bool { return false }

func (c *JSONCollector) Events() []Event {
	return c.events
}

func (c *JSONCollector) FlushReport(meta ReportMeta, exitCode int, exitReason, finalContent string, usage any) error {
	rep := RunReport{
		V:            SchemaVersion,
		Prompt:       meta.Prompt,
		Model:        meta.Model,
		Provider:     meta.Provider,
		ProjHex:      meta.ProjHex,
		Ephemeral:    meta.Ephemeral,
		Events:       c.events,
		ExitCode:     exitCode,
		ExitReason:   exitReason,
		FinalContent: finalContent,
		Usage:        usage,
	}
	if exitCode != ExitOK {
		rep.Error = exitReason
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.out.Write(b)
	return err
}
