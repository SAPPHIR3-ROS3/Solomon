//go:build automatic_role_scores

package scoring

import (
	"math"
	"sort"
)

const gapEpsilon = 1e-9

func GapAwareNormalize(values []float64, higherIsBetter bool) []float64 {
	n := len(values)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []float64{100}
	}
	type item struct {
		idx int
		v   float64
	}
	items := make([]item, n)
	for i, v := range values {
		nv := v
		if !higherIsBetter {
			nv = -v
		}
		items[i] = item{idx: i, v: nv}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].v != items[j].v {
			return items[i].v > items[j].v
		}
		return items[i].idx < items[j].idx
	})
	top := items[0].v
	bottom := items[n-1].v
	span := top - bottom
	out := make([]float64, n)
	if span <= gapEpsilon {
		for i := range out {
			out[i] = 100
		}
		return out
	}
	out[items[0].idx] = 100
	cum := 0.0
	for i := 1; i < n; i++ {
		gap := items[i-1].v - items[i].v
		if gap < 0 {
			gap = 0
		}
		cum += gap
		score := 100 * (1 - cum/span)
		if score < 0 {
			score = 0
		}
		out[items[i].idx] = score
	}
	return out
}

func RoundScore(v float64) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(math.Round(v))
}
