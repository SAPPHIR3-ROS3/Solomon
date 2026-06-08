//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	yGridStep    = 5000
	minSlotW     = 10.0
	minPlotW     = 1100.0
	viewHMin     = 520.0
	viewHOverW   = 0.28
	viewAspectWH      = 2.0277
	viewportH         = "98vh"
	chartML           = 12.0
	chartMR           = 12.0
	chartMT           = 16.0
	legendPadFromPlot = 40.0
	legendHeight      = 20.0
	chartBottomPad    = 24.0
)

type chartLayout struct {
	w, h  int
	plotH float64
	ml    float64
	mr    float64
	mt    float64
	mb    float64
}

func chartSize(n int) chartLayout {
	plotW := float64(n) * minSlotW
	if plotW < minPlotW {
		plotW = minPlotW
	}
	w := int(chartML + chartMR + plotW)
	targetH := float64(w) / viewAspectWH
	minH := float64(w) * viewHOverW
	if minH < viewHMin {
		minH = viewHMin
	}
	if targetH < minH {
		targetH = minH
		w = int(targetH * viewAspectWH)
	}
	h := int(targetH)
	mb := legendPadFromPlot + legendHeight + chartBottomPad
	plotH := targetH - chartMT - mb
	return chartLayout{
		w: w, h: h, plotH: plotH,
		ml: chartML, mr: chartMR, mt: chartMT, mb: mb,
	}
}

func writeChart(path string, stats []commitStat) error {
	n := len(stats)
	lay := chartSize(n)
	svgW, svgH, plotH := lay.w, lay.h, lay.plotH

	maxY := 1
	lastLOC := 0
	for i, s := range stats {
		if s.totalLOC > maxY {
			maxY = s.totalLOC
		}
		if s.insertions > maxY {
			maxY = s.insertions
		}
		if s.deletions > maxY {
			maxY = s.deletions
		}
		if i == len(stats)-1 {
			lastLOC = s.totalLOC
		}
	}
	maxY = ceilToStep(maxY, yGridStep)

	pX0, pX1 := lay.ml, float64(svgW)-lay.mr
	pY0, pY1 := lay.mt, lay.mt+plotH
	pW, pH := pX1-pX0, pY1-pY0

	yFor := func(v int) float64 {
		if maxY <= 0 {
			return pY1
		}
		return pY1 - float64(v)*pH/float64(maxY)
	}

	var b strings.Builder
	b.Grow(64 * 1024)
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="100%%" height="%s" viewBox="0 0 %d %d" preserveAspectRatio="xMidYMid meet" style="display:block;width:100%%;max-width:100%%;height:%s">`, viewportH, svgW, svgH, viewportH)
	fmt.Fprint(&b, `<rect width="100%" height="100%" fill="#f8fafc"/>`)
	fmt.Fprint(&b, `<style>
  .title { font: 600 22px system-ui, Segoe UI, sans-serif; fill: #0f172a; }
  .axis { font: 16px system-ui, Segoe UI, sans-serif; fill: #334155; }
  .tick { font: 14px system-ui, Segoe UI, sans-serif; fill: #475569; }
  .legend { font: 15px system-ui, Segoe UI, sans-serif; fill: #0f172a; }
  .ref-label { font: 600 14px system-ui, Segoe UI, sans-serif; fill: #1d4ed8; }
</style>`)
	fmt.Fprintf(&b, `<text class="title" x="%.0f" y="32">LOC totali (sfondo) con aggiunte e rimosse sovrapposte</text>`, pX0)

	for v := 0; v <= maxY; v += yGridStep {
		y := yFor(v)
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#e2e8f0"/>`, pX0, y, pX1, y)
		fmt.Fprintf(&b, `<text class="axis" x="%.1f" y="%.1f" text-anchor="end" dominant-baseline="middle">%s</text>`, pX0-10, y, xmlEsc(formatK(v)))
	}
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#64748b"/>`, pX0, pY1, pX1, pY1)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#64748b"/>`, pX0, pY0, pX0, pY1)

	slotW := pW / float64(n)
	if slotW < 3 {
		slotW = 3
	}
	barW := slotW * 0.92
	if barW < 6 {
		barW = 6
	}

	barH := func(v int) float64 {
		h := float64(v) * pH / float64(maxY)
		if h < 1 && v > 0 {
			return 1
		}
		return h
	}

	for i, s := range stats {
		cx := pX0 + (float64(i)+0.5)*slotW
		x0 := cx - barW/2

		locH := barH(s.totalLOC)
		fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#93c5fd" stroke="#2563eb" stroke-width="0.5"/>`, x0, pY1-locH, barW, locH)

		addH := barH(s.insertions)
		delH := barH(s.deletions)
		if s.deletions > 0 {
			fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#dc2626"/>`, x0, pY1-delH, barW, delH)
		}
		if s.insertions > 0 {
			fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#16a34a"/>`, x0, pY1-delH-addH, barW, addH)
		}
	}

	if n > 0 && lastLOC > 0 {
		refY := yFor(lastLOC)
		refLbl := formatExact(lastLOC) + " righe (ultimo commit)"
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.2f" x2="%.1f" y2="%.2f" stroke="#1d4ed8" stroke-width="2" stroke-dasharray="8 6"/>`, pX0, refY, pX1, refY)
		fmt.Fprintf(&b, `<text class="ref-label" x="%.1f" y="%.0f" text-anchor="end">%s</text>`, pX1, pY0-6, xmlEsc(refLbl))
	}

	const tickLen = 10.0
	labelY := pY1 + tickLen + 14
	for _, num := range commitLabelNums(n) {
		cx := barCenterX(pX0, pW, n, num)
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#94a3b8" stroke-width="1"/>`, cx, pY1, cx, pY1+tickLen)
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%.1f" text-anchor="middle">%d</text>`, cx, labelY, num)
	}

	ly := pY1 + 44
	xLeg := pX0
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="20" height="20" fill="#93c5fd" stroke="#2563eb" stroke-width="0.5" rx="2"/>`, xLeg, ly)
	fmt.Fprintf(&b, `<text class="legend" x="%.0f" y="%.0f" dominant-baseline="middle">LOC totali (sfondo)</text>`, xLeg+28, ly+10)
	xLeg += 190
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="20" height="20" fill="#dc2626" rx="2"/>`, xLeg, ly)
	fmt.Fprintf(&b, `<text class="legend" x="%.0f" y="%.0f" dominant-baseline="middle">rimosse (sotto)</text>`, xLeg+28, ly+10)
	xLeg += 180
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="20" height="20" fill="#16a34a" rx="2"/>`, xLeg, ly)
	fmt.Fprintf(&b, `<text class="legend" x="%.0f" y="%.0f" dominant-baseline="middle">aggiunte (sopra)</text>`, xLeg+28, ly+10)
	xLeg += 180
	fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#1d4ed8" stroke-width="2" stroke-dasharray="8 6"/>`, xLeg, ly+10, xLeg+36, ly+10)
	fmt.Fprintf(&b, `<text class="legend" x="%.0f" y="%.0f" dominant-baseline="middle">LOC ultimo commit</text>`, xLeg+44, ly+10)
	fmt.Fprintf(&b, `<text class="axis" x="%.0f" y="%.0f" text-anchor="middle">numero commit</text>`, (pX0+pX1)/2, pY1+lay.mb-6)
	fmt.Fprint(&b, `</svg>`)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func formatExact(v int) string {
	if v < 0 {
		return "-" + formatExact(-v)
	}
	s := strconv.Itoa(v)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	if s != "" {
		parts = append([]string{s}, parts...)
	}
	return strings.Join(parts, ".")
}

func commitLabelNums(n int) []int {
	if n <= 0 {
		return nil
	}
	var nums []int
	for v := 10; v < n; v += 10 {
		nums = append(nums, v)
	}
	if len(nums) == 0 || nums[len(nums)-1] != n {
		nums = append(nums, n)
	}
	return nums
}

func barCenterX(pX0, pW float64, n, commitNum int) float64 {
	idx := commitNum - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	slotW := pW / float64(n)
	if slotW < 3 {
		slotW = 3
	}
	return pX0 + (float64(idx)+0.5)*slotW
}

func ceilToStep(v, step int) int {
	if step <= 0 {
		return v
	}
	if v <= 0 {
		return step
	}
	return ((v + step - 1) / step) * step
}

func formatK(v int) string {
	if v >= 1000 {
		return fmt.Sprintf("%dk", (v+500)/1000)
	}
	return strconv.Itoa(v)
}
