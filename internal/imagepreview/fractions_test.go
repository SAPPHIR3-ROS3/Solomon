package imagepreview

import "testing"

func fractionError(p []rgb, ch rune, fg, bg rgb) float64 {
	for _, g := range fractionGlyphs {
		if g.ch == ch {
			cost := 0.0
			for i, v := range p {
				target := bg
				if g.mask&(uint64(1)<<i) != 0 {
					target = fg
				}
				cost += distance(v, target)
			}
			return cost
		}
	}
	panic("unknown fraction")
}
func TestFractionsPreserveTheirExactGeometry(t *testing.T) {
	seen := map[uint64]bool{}
	for _, g := range fractionGlyphs {
		if seen[g.mask] {
			t.Fatal("duplicate fraction")
		}
		seen[g.mask] = true
		p := make([]rgb, 64)
		for i := range p {
			p[i] = rgb{0, 0, 255}
			if g.mask&(uint64(1)<<i) != 0 {
				p[i] = rgb{255, 0, 0}
			}
		}
		ch, fg, bg := fractionCell(p, TrueColor)
		if fractionError(p, ch, fg, bg) != 0 {
			t.Fatalf("lost fraction %U", g.ch)
		}
	}
	if len(seen) != 17 {
		t.Fatalf("unexpected repertoire size: %d", len(seen))
	}
}
func TestFractionsNeverWorseThanHorizontalHalves(t *testing.T) {
	p := make([]rgb, 64)
	for i := range p {
		p[i] = rgb{float64((i * 37) % 256), float64((i * 11) % 256), float64((i * 19) % 256)}
	}
	for _, colors := range []Colors{TrueColor, ANSI256, None} {
		ch, fg, bg := fractionCell(p, colors)
		cost := fractionError(p, ch, fg, bg)
		_, _, baseline := fitMask(p, rectangleMask(0, 0, 8, 4), colors)
		if cost > baseline+1e-6 {
			t.Fatal("fraction selection lost half-block candidate")
		}
		if colors != None && (fg != quantize(fg, colors) || bg != quantize(bg, colors)) {
			t.Fatal("scored colors differ from output colors")
		}
	}
}
