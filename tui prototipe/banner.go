package main

import (
	"math"
	"strings"
)

const (
	logoSaturateMul    = 1.38
	logoContrastMul    = 1.14
	logoYellowHueLo    = 0.065
	logoYellowHueHi    = 0.23
	logoYellowMinSat   = 0.05
	logoYellowSatExtra = 1.48
	logoYellowBrighten = 0.11
)

const originalLogo = "              ⣤⣤             \n             ⡷⢾⡷⢾            \n            ⣀⣴⣾⣷⣦⣀           \n    ⣠⣶⣶⠿⢛⣿⡿⠛⠉ ⣿⣿ ⠉⠛⢿⣿⡛⠿⣶⣶⣄   \n  ⢠⣾⠟⠉⢀⣤⣿⣿⣾⣿⣿⣿⣿⣿⣿⣿⣿⣷⣿⣿⣤⡀⠉⠻⣷⡄ \n  ⣾⣏⣴⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣦⣹⣷ \n  ⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿ \n  ⠸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠇ \n   ⠈⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠁  \n    ⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿   \n      ⠉⠉⠉⠉⠛⠛⠛⠛⠛⠛⠛⠛⠛⠉⠉⠉⠉       "
const originalColors = "000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 ffcb58 ffc704 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000\n000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 ffcb58 ffcf2e ffcf2e df9f03 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000\n000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 ffcb58 ffcb58 ffcf2e ffc704 df9f03 df9f03 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000\n000000 000000 000000 000000 ffcf2e ffcf2e ffcf2e ffcf2e ffcf2e ffc704 ffc704 ffc704 ffc704 000000 ffc704 ffc704 000000 ffc704 ffc704 ffc704 ffc704 ffc704 df9f03 df9f03 df9f03 df9f03 df9f03 000000 000000 000000\n000000 000000 ffcf2e ffcf2e ffcf2e ffc704 920905 920905 ffcf2e ffc704 820906 920905 a30904 b30903 ffc704 efb303 a30904 a30904 b30903 d40801 ffc704 efb303 d40801 d40801 df9f03 df9f03 df9f03 cf8b02 000000 000000\n000000 000000 ffcf2e ffc704 820906 920905 b30903 d40801 ffcf2e df9f03 710a07 d40801 d40801 d40801 ffc704 efb303 610a08 d40801 d40801 d40801 ffc704 efb303 a30904 b30903 c40802 d40801 df9f03 cf8b02 000000 000000\n000000 000000 ffcf2e ffc704 710a07 d40801 d40801 ffcf2e ffc704 efb303 cf8b02 d40801 d40801 ffc704 ffc704 efb303 cf8b02 d40801 d40801 ffc704 ffc704 efb303 cf8b02 d40801 d40801 d40801 df9f03 cf8b02 000000 000000\n000000 000000 ffcf2e ffc704 610a08 ffc704 ffcf2e ffc704 ffc704 ffc704 efb303 df9f03 df9f03 ffc704 ffc704 ffc704 efb303 df9f03 df9f03 ffc704 ffc704 ffc704 efb303 df9f03 df9f03 d40801 cf8b02 cf8b02 000000 000000\n000000 000000 000000 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 ffc704 cf8b02 cf8b02 000000 000000 000000\n000000 000000 000000 000000 ffc704 610a08 710a07 820906 920905 a30904 b30903 c40802 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 cf8b02 000000 000000 000000 000000\n000000 000000 000000 000000 000000 610a08 710a07 820906 920905 a30904 b30903 cc0802 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 d40801 000000 000000 000000 000000\n"

func banner() string {
	rows := strings.Split(strings.TrimRight(originalLogo, "\n"), "\n")
	colors := strings.Split(strings.TrimRight(originalColors, "\n"), "\n")
	for i, row := range rows {
		fields := []string{}
		if i < len(colors) {
			fields = strings.Fields(colors[i])
		}
		var out strings.Builder
		for j, ch := range []rune(strings.TrimRight(row, " \u2800")) {
			if j < len(fields) {
				r, g, b := enhanceLogoRGB(parseHexRGB(fields[j]))
				out.WriteString(text(string(ch), fmtColor(r, g, b)))
			} else {
				out.WriteRune(ch)
			}
		}
		rows[i] = out.String()
	}
	return strings.Join(rows, "\n")
}
func fmtColor(r, g, b uint8) string {
	const digits = "0123456789abcdef"
	return string([]byte{'#', digits[r>>4], digits[r&15], digits[g>>4], digits[g&15], digits[b>>4], digits[b&15]})
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
