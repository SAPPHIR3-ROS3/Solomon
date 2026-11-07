package memoryprofile

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"
)

const SampleLimit = 60
const Interval = 10 * time.Second

type Usage struct {
	Processes int    `json:"processes"`
	RSS       uint64 `json:"rssBytes"`
	Swap      uint64 `json:"swapBytes"`
}

type Sample struct {
	AgeSeconds  int64  `json:"ageSeconds"`
	Server      Usage  `json:"server"`
	Desktop     Usage  `json:"desktop"`
	WebKit      Usage  `json:"webkit"`
	Heap        uint64 `json:"goHeapBytes"`
	HeapObjects uint64 `json:"goHeapObjects"`
	GC          uint32 `json:"goGCCount"`
}

type Report struct {
	Scope                   string   `json:"scope"`
	ProcessMetricsAvailable bool     `json:"processMetricsAvailable"`
	IntervalSeconds         int      `json:"intervalSeconds"`
	PeakRSS                 uint64   `json:"peakObservedRSSBytes"`
	Samples                 []Sample `json:"samples"`
}

type Collector struct {
	mu        sync.Mutex
	started   time.Time
	samples   []Sample
	peak      uint64
	available bool
	home      string
}

var collectProcesses = func(string) (Usage, Usage, Usage, bool) { return Usage{}, Usage{}, Usage{}, false }
var Default = New("")

func New(home string) *Collector { return &Collector{home: home, started: time.Now()} }

func (c *Collector) Sample() {
	server, desktop, webkit, available := collectProcesses(c.home)
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	c.mu.Lock()
	defer c.mu.Unlock()
	sample := Sample{AgeSeconds: int64(time.Since(c.started).Seconds()), Server: server, Desktop: desktop, WebKit: webkit, Heap: stats.HeapAlloc, HeapObjects: stats.HeapObjects, GC: stats.NumGC}
	c.available = available
	c.peak = max(c.peak, server.RSS+desktop.RSS+webkit.RSS)
	if len(c.samples) == SampleLimit {
		copy(c.samples, c.samples[1:])
		c.samples = c.samples[:SampleLimit-1]
	}
	c.samples = append(c.samples, sample)
}

func (c *Collector) Report() Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Report{Scope: "solomon-owned-processes", ProcessMetricsAvailable: c.available, IntervalSeconds: int(Interval.Seconds()), PeakRSS: c.peak, Samples: append([]Sample{}, c.samples...)}
}

func (c *Collector) JSON() string {
	data, _ := json.Marshal(c.Report())
	return string(data)
}

func (c *Collector) Run(ctx context.Context) {
	c.Sample()
	ticker := time.NewTicker(Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Sample()
		}
	}
}
