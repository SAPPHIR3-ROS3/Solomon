package test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestReportBudgetKeepsNewestSamplesAndPeak(t *testing.T) {
	usage := memoryprofile.Usage{Processes: 256, RSS: ^uint64(0), Swap: ^uint64(0)}
	report := memoryprofile.Report{Scope: "solomon-owned-processes", ProcessMetricsAvailable: true, IntervalSeconds: 10, PeakRSS: ^uint64(0)}
	for i := 0; i < 60; i++ {
		report.Samples = append(report.Samples, memoryprofile.Sample{AgeSeconds: int64(i), Server: usage, Desktop: usage, WebKit: usage, Heap: ^uint64(0), HeapObjects: ^uint64(0), GC: ^uint32(0)})
	}
	metrics, err := report.BoundedJSON(memoryprofile.ReportByteLimit)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(metrics)
	if len(wire) > 16*1024 {
		t.Fatalf("memory budget: %d", len(wire))
	}
	var trimmed memoryprofile.Report
	if err := json.Unmarshal([]byte(metrics), &trimmed); err != nil {
		t.Fatal(err)
	}
	if len(trimmed.Samples) == 0 || len(trimmed.Samples) >= 60 || trimmed.Samples[len(trimmed.Samples)-1].AgeSeconds != 59 || trimmed.PeakRSS != report.PeakRSS {
		t.Fatalf("trimmed metrics: %+v", trimmed)
	}
	if len(report.Samples) != 60 || report.Samples[0].AgeSeconds != 0 {
		t.Fatal("source history was modified")
	}
	if _, err := report.BoundedJSON(1); err == nil {
		t.Fatal("expected impossible budget error")
	}
	for _, logs := range []string{
		strings.Repeat("plain log\n", 4000),
		strings.Repeat("\x00\"\\😀\n", 4000),
	} {
		logs += "newest log entry"
		payload, err := server.BuildReportPayloadForTest(strings.Repeat("😀", 500), logs, report)
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) > 48*1024 || !utf8.Valid(payload) {
			t.Fatalf("report size or encoding: %d", len(payload))
		}
		var decoded map[string]string
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(decoded["logs"], "newest log entry") || !utf8.ValidString(decoded["logs"]) {
			t.Fatal("newest Unicode log text was damaged")
		}
		encodedMetrics, _ := json.Marshal(decoded["memory"])
		if len(encodedMetrics) > memoryprofile.ReportByteLimit {
			t.Fatal("metrics overflow")
		}
		t.Logf("payload=%d bytes, memory=%d bytes, samples=%d", len(payload), len(encodedMetrics), len(trimmed.Samples))
	}
	if logging.ReportByteLimit != 48*1024 || logging.ReportLogByteLimit != 32*1024 {
		t.Fatal("incorrect report allocation")
	}
}
