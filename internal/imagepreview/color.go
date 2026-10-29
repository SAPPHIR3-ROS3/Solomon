package imagepreview

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// DetectColors is a best-effort capability hint, never a terminal query. The
// caller controls TTY detection and may explicitly select a color mode instead.
func DetectColors(terminal bool) Colors {
	if !terminal || os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || os.Getenv("TERM") == "dumb" {
		return None
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	t := strings.ToLower(os.Getenv("TERM"))
	if ct == "truecolor" || ct == "24bit" || strings.Contains(t, "direct") {
		return TrueColor
	}
	return ANSI256
}

func colorStyle(fg, bg rgb, c Colors) string {
	if c == TrueColor {
		return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm", byteColor(fg.r), byteColor(fg.g), byteColor(fg.b), byteColor(bg.r), byteColor(bg.g), byteColor(bg.b))
	}
	return fmt.Sprintf("\x1b[38;5;%d;48;5;%dm", paletteIndex(fg), paletteIndex(bg))
}

// Restrict to the stable xterm cube and grayscale ramp: indices 0..15 are
// user-theme dependent. Weighted RGB error selects cube vs gray objectively.
func paletteIndex(c rgb) int {
	levels := [6]float64{0, 95, 135, 175, 215, 255}
	nearest := func(v float64) int {
		best := 0
		for i := 1; i < 6; i++ {
			if math.Abs(v-levels[i]) < math.Abs(v-levels[best]) {
				best = i
			}
		}
		return best
	}
	r, g, b := nearest(c.r), nearest(c.g), nearest(c.b)
	index := 16 + 36*r + 6*g + b
	best := distance(c, rgb{levels[r], levels[g], levels[b]})
	gray := max(0, min(23, int(math.Round((c.luma()-8)/10))))
	v := float64(8 + 10*gray)
	if distance(c, rgb{v, v, v}) < best {
		return 232 + gray
	}
	return index
}

// Quantize before scoring candidates so ANSI-256 selection optimizes the colors
// that will actually be emitted, not an unavailable continuous RGB value.
func quantize(c rgb, colors Colors) rgb {
	if colors == TrueColor {
		return rgb{float64(byteColor(c.r)), float64(byteColor(c.g)), float64(byteColor(c.b))}
	}
	if colors == ANSI256 {
		return paletteColor(paletteIndex(c))
	}
	return c
}
func paletteColor(n int) rgb {
	if n >= 232 {
		v := float64(8 + (n-232)*10)
		return rgb{v, v, v}
	}
	levels := [6]float64{0, 95, 135, 175, 215, 255}
	n -= 16
	return rgb{levels[n/36], levels[(n/6)%6], levels[n%6]}
}
