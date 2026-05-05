package logo

import (
	_ "embed"
	"math"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"
)

const (
	logoSaturateMul      = 1.38
	logoContrastMul      = 1.14
	logoYellowHueLo      = 0.065
	logoYellowHueHi      = 0.23
	logoYellowMinSat     = 0.05
	logoYellowSatExtra   = 1.48
	logoYellowBrighten   = 0.11
)

//go:embed colors.txt
var ColorsRaw string

type LogoLine struct {
	Plain string
	ANSI  string
}

func WelcomeLogoLines() []LogoLine {
	raw := strings.ReplaceAll(ASCII, "\r\n", "\n")
	txtParts := strings.Split(strings.TrimRight(raw, "\n"), "\n")

	colorRaw := strings.ReplaceAll(ColorsRaw, "\r\n", "\n")
	colorParts := strings.Split(strings.TrimRight(colorRaw, "\n"), "\n")

	out := make([]LogoLine, len(txtParts))
	for i, txtLine := range txtParts {
		runes := []rune(txtLine)
		hexes := parseColorRow(colorParts[i])

		var ab strings.Builder
		for j, r := range runes {
			if j < len(hexes) && hexes[j] != "" {
				r2, g2, b2 := enhanceLogoRGB(parseHexRGB(hexes[j]))
				ab.WriteString(termcolor.ForegroundRGB(r2, g2, b2))
			}
			ab.WriteRune(r)
		}

		ansiRaw := ab.String() + termcolor.Reset
		// trimma a destra i Braille blank e spazi (sia da Plain che da ANSI)
		plainTrimmed := strings.TrimRightFunc(txtLine, func(r rune) bool {
			return r == '\u2800' || r == ' '
		})

		// Rimuove prima il Reset finale per poter trimmare i blank sottostanti.
		ansiBody := strings.TrimSuffix(ansiRaw, termcolor.Reset)
		ansiTrimmed := trimANSIRight(ansiBody) + termcolor.Reset

		out[i] = LogoLine{Plain: plainTrimmed, ANSI: ansiTrimmed}
	}
	return out
}

// trimANSIRight rimuove i caratteri Braille blank (U+2800) e spazi dalla fine di una stringa ANSI,
// insieme alle sequenze di colore che li precedono. La stringa NON deve includere il Reset finale.
func trimANSIRight(s string) string {
	runes := []rune(s)
	end := len(runes)

	for end > 0 {
		pos := end - 1

		// Il carattere più a destra: se è blank/spazio, lo rimuoviamo
		r := runes[pos]
		if r == '\u2800' || r == ' ' {
			end-- // rimuove il blank
			// Ora salta la sequenza escape che precede questo blank (es. \x1b[38;2;R;G;Bm)
			for end > 0 && runes[end-1] != '\x1b' {
				end--
			}
			if end > 0 && runes[end-1] == '\x1b' {
				end-- // rimuove anche \x1b
			}
			continue
		}

		// Se siamo su una sequenza escape senza blank dopo, fermiamoci
		break
	}

	return string(runes[:end])
}

func parseColorRow(line string) []string {
	fields := strings.Fields(strings.TrimSpace(line))
	hexes := make([]string, len(fields))
	for i, f := range fields {
		hexes[i] = f
	}
	return hexes
}

func parseHexRGB(hex string) (r, g, b uint8) {
	if len(hex) != 6 {
		return 0x80, 0x80, 0x80
	}
	parseByte := func(off int) byte {
		h := hex[off : off+2]
		var v byte
		for _, c := range h {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v += byte(c - '0')
			case c >= 'a' && c <= 'f':
				v += byte(c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				v += byte(c - 'A' + 10)
			}
		}
		return v
	}
	return parseByte(0), parseByte(2), parseByte(4)
}

func enhanceLogoRGB(r, g, b uint8) (uint8, uint8, uint8) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255
	h, s, l := rgbToHSL(rf, gf, bf)
	s *= logoSaturateMul
	if s > 1 {
		s = 1
	}
	if s >= logoYellowMinSat && h >= logoYellowHueLo && h <= logoYellowHueHi {
		s *= logoYellowSatExtra
		if s > 1 {
			s = 1
		}
		l += (1 - l) * logoYellowBrighten
		if l > 0.94 {
			l = 0.94
		}
	}
	rf, gf, bf = hslToRGB(h, s, l)
	r2 := clampU8Float(rf * 255)
	g2 := clampU8Float(gf * 255)
	b2 := clampU8Float(bf * 255)
	c := logoContrastMul
	return clampU8Float(128 + c*float64(int(r2)-128)),
		clampU8Float(128 + c*float64(int(g2)-128)),
		clampU8Float(128 + c*float64(int(b2)-128))
}

func clampU8Float(x float64) uint8 {
	if x <= 0 {
		return 0
	}
	if x >= 255 {
		return 255
	}
	return uint8(math.Round(x))
}

func rgbToHSL(r, g, b float64) (h, s, l float64) {
	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}
	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}
	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if b > g {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h /= 6
	return h, s, l
}

func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r = hueToRGB(p, q, h+1.0/3)
	g = hueToRGB(p, q, h)
	b = hueToRGB(p, q, h-1.0/3)
	return r, g, b
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6 {
		return p + (q-p)*6*t
	}
	if t < 0.5 {
		return q
	}
	if t < 2.0/3 {
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}
