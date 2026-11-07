package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
)

func TestMemoryProfileBoundedAndAnonymous(t *testing.T) {
	collector := memoryprofile.New(t.TempDir())
	for i := 0; i < memoryprofile.SampleLimit+5; i++ {
		collector.Sample()
	}
	report := collector.Report()
	if len(report.Samples) != memoryprofile.SampleLimit {
		t.Fatalf("samples: %d", len(report.Samples))
	}
	encoded := collector.JSON()
	for _, forbidden := range []string{"pid", "executable", "identity", "username", "path", "content"} {
		if strings.Contains(strings.ToLower(encoded), forbidden) {
			t.Fatalf("private field: %s", forbidden)
		}
	}
	if report.Samples[0].Heap == 0 {
		t.Fatal("missing heap measurement")
	}
	report.Samples[0].Heap = 0
	if collector.Report().Samples[0].Heap == 0 {
		t.Fatal("report aliases collector history")
	}
	payload := map[string]string{"memory": encoded}
	wire, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("60 samples: raw=%d bytes, JSON report contribution=%d bytes", len(encoded), len(wire))
	if len(wire) > 32*1024 {
		t.Fatalf("memory section exceeds 32 KiB: %d", len(wire))
	}
}

func TestMemoryProfileWorstCaseBudget(t *testing.T) {
	usage := memoryprofile.Usage{Processes: 256, RSS: ^uint64(0), Swap: ^uint64(0)}
	sample := memoryprofile.Sample{AgeSeconds: 9999999999, Server: usage, Desktop: usage, WebKit: usage, Heap: ^uint64(0), HeapObjects: ^uint64(0), GC: ^uint32(0)}
	report := memoryprofile.Report{Scope: "solomon-owned-processes", ProcessMetricsAvailable: true, IntervalSeconds: 10, PeakRSS: ^uint64(0)}
	for i := 0; i < memoryprofile.SampleLimit; i++ {
		report.Samples = append(report.Samples, sample)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]string{"memory": string(raw)})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("populated upper bound: raw=%d bytes, escaped=%d bytes", len(raw), len(encoded))
	if len(encoded) > 32*1024 {
		t.Fatalf("memory budget exceeded: %d", len(encoded))
	}
}
