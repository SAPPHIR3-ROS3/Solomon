//go:build automatic_role_scores

package scoring

import (
	"math"
	"sort"
)

func AggregateF2(scores []float64) float64 {
	n := len(scores)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return scores[0]
	}
	if n == 2 {
		return (scores[0] + scores[1]) / 2
	}
	sorted := append([]float64(nil), scores...)
	sort.Float64s(sorted)
	gaps := make([]float64, n-1)
	allZero := true
	for i := 0; i < n-1; i++ {
		gaps[i] = sorted[i+1] - sorted[i]
		if gaps[i] > gapEpsilon {
			allZero = false
		}
	}
	if allZero {
		sum := 0.0
		for _, s := range sorted {
			sum += s
		}
		return sum / float64(n)
	}
	type window struct {
		localGap float64
		start    int
		end      int
	}
	windows := make([]window, 0, n-1)
	for w := 0; w < n-1; w++ {
		var local float64
		switch {
		case n == 2:
			local = gaps[0]
		case w == 0:
			local = (gaps[0] + gaps[1]) / 2
		case w == n-2:
			local = (gaps[n-3] + gaps[n-2]) / 2
		default:
			local = (gaps[w-1] + gaps[w]) / 2
		}
		if local < gapEpsilon {
			local = gapEpsilon
		}
		windows = append(windows, window{localGap: local, start: w, end: w + 1})
	}
	inv := make([]float64, len(windows))
	for i, w := range windows {
		inv[i] = 1 / w.localGap
	}
	weights := softmax(inv)
	var sum float64
	var wSum float64
	for i, w := range windows {
		count := float64(w.end - w.start + 1)
		part := 0.0
		for j := w.start; j <= w.end; j++ {
			part += sorted[j]
		}
		part /= count
		sum += weights[i] * part
		wSum += weights[i]
	}
	if wSum <= 0 {
		avg := 0.0
		for _, s := range sorted {
			avg += s
		}
		return avg / float64(n)
	}
	return sum / wSum
}

func ConsistencyFromSources(scores []float64) int {
	n := len(scores)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return 100
	}
	minV, maxV := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < minV {
			minV = s
		}
		if s > maxV {
			maxV = s
		}
	}
	spread := maxV - minV
	if spread <= gapEpsilon {
		return 100
	}
	v := 100 * (1 - spread/100)
	return RoundScore(v)
}

func softmax(xs []float64) []float64 {
	if len(xs) == 0 {
		return nil
	}
	max := xs[0]
	for _, x := range xs[1:] {
		if x > max {
			max = x
		}
	}
	sum := 0.0
	out := make([]float64, len(xs))
	for i, x := range xs {
		e := math.Exp(x - max)
		out[i] = e
		sum += e
	}
	if sum <= 0 {
		u := 1.0 / float64(len(xs))
		for i := range out {
			out[i] = u
		}
		return out
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}
