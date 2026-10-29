package imagepreview

import "math"

// fitMask uses the best two mean colors for a fixed partition. In monochrome,
// the same geometry is matched against white ink on a black background.
func fitMask(p []rgb, mask uint64, colors Colors) (rgb, rgb, float64) {
	var fg, bg rgb
	nf, nb := 0, 0
	for i, v := range p {
		if mask&(uint64(1)<<i) != 0 {
			fg = fg.add(v)
			nf++
		} else {
			bg = bg.add(v)
			nb++
		}
	}
	if colors == None {
		fg = rgb{255, 255, 255}
		bg = rgb{}
	} else {
		if nf > 0 {
			fg = fg.mul(1 / float64(nf))
		}
		if nb > 0 {
			bg = bg.mul(1 / float64(nb))
		}
		fg = quantize(fg, colors)
		bg = quantize(bg, colors)
	}
	cost := 0.0
	for i, v := range p {
		target := bg
		if mask&(uint64(1)<<i) != 0 {
			target = fg
		}
		cost += distance(v, target)
	}
	return fg, bg, cost
}

type blockGlyph struct {
	ch   rune
	mask uint64
}

func rectangleMask(x0, y0, x1, y1 int) uint64 {
	var mask uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			mask |= uint64(1) << (y*8 + x)
		}
	}
	return mask
}

// Fractions include horizontal half blocks and eighth strips. The fixed
// repertoire keeps preview quality predictable without selectable backends.
var fractionGlyphs = func() []blockGlyph {
	out := []blockGlyph{{'▀', rectangleMask(0, 0, 8, 4)}, {'▄', rectangleMask(0, 4, 8, 8)}, {' ', 0}, {'█', ^uint64(0)}}
	for k := 1; k <= 8; k++ {
		out = append(out, blockGlyph{rune(0x2580 + k), rectangleMask(0, 8-k, 8, 8)})
	}
	for k := 1; k < 8; k++ {
		out = append(out, blockGlyph{rune(0x2590 - k), rectangleMask(0, 0, k, 8)})
	}
	seen := map[uint64]bool{}
	unique := out[:0]
	for _, g := range out {
		if !seen[g.mask] {
			seen[g.mask] = true
			unique = append(unique, g)
		}
	}
	return unique
}()

func fractionCell(p []rgb, colors Colors) (rune, rgb, rgb) {
	best := math.Inf(1)
	ch := ' '
	var fg, bg rgb
	for _, g := range fractionGlyphs {
		f, b, cost := fitMask(p, g.mask, colors)
		if cost < best {
			best = cost
			ch, fg, bg = g.ch, f, b
		}
	}
	return ch, fg, bg
}
