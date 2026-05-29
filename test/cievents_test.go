package test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
)

func TestJSONLEmitterOneLinePerEvent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	em := cievents.NewJSONLEmitter(&buf)
	em.Emit(cievents.RunStart("hi", "m", "p", "hex", false))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("lines %d", len(lines))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatal(err)
	}
	if m["type"] != cievents.TypeRunStart {
		t.Fatalf("%v", m)
	}
}

func TestJSONCollectorFlushReport(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	col := cievents.NewJSONCollector(&buf)
	col.Emit(cievents.RunEnd(0, "ok", "done", nil))
	meta := cievents.ReportMeta{Prompt: "p", Model: "m", Provider: "ci-env"}
	if err := col.FlushReport(meta, 0, "ok", "done", nil); err != nil {
		t.Fatal(err)
	}
	var rep cievents.RunReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.ExitCode != 0 || rep.FinalContent != "done" || len(rep.Events) != 1 {
		t.Fatalf("%+v", rep)
	}
}

func TestClassifyExitToolPolicy(t *testing.T) {
	t.Parallel()
	code, _ := cievents.ClassifyExit(cievents.ToolPolicyError())
	if code != cievents.ExitTool {
		t.Fatalf("code %d", code)
	}
}
