package logo

import (
	_ "embed"
	"math"
	"regexp"
	"strings"
	"unicode"

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

//go:embed map.html
var logoMapHTML string

type LogoLine struct {
	Plain string
	ANSI  string
}

var (
	reSpanOpen = regexp.MustCompile(`<span\s+style="color:#([0-9a-fA-F]{6})"\s*>`)
	reBr       = regexp.MustCompile(`(?i)<br\s*/?>`)
	reAnyTag   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func WelcomeLogoLines() []LogoLine {
	lines := parseLogoHTML(logoMapHTML)
	if len(lines) == 0 {
		raw := strings.ReplaceAll(ASCII, "\r\n", "\n")
		parts := strings.Split(strings.TrimRight(raw, "\n"), "\n")
		out := make([]LogoLine, len(parts))
		for i, p := range parts {
			out[i] = LogoLine{Plain: p, ANSI: p}
		}
		return out
	}
	return lines
}

func parseLogoHTML(html string) []LogoLine {
	body := html
	if i := strings.Index(strings.ToLower(html), "<body"); i >= 0 {
		if j := strings.Index(html[i:], ">"); j >= 0 {
			body = html[i+j+1:]
		}
	}
	if i := strings.Index(strings.ToLower(body), "</body>"); i >= 0 {
		body = body[:i]
	}
	rawLines := reBr.Split(body, -1)
	lines := make([]LogoLine, 0, len(rawLines))
	for _, frag := range rawLines {
		plain, ansi := renderColoredFragments(frag)
		lines = append(lines, LogoLine{Plain: plain, ANSI: ansi})
	}
	for len(lines) > 0 {
		last := lines[len(lines)-1]
		if !isVisualPaddingLine(last.Plain) {
			break
		}
		lines = lines[:len(lines)-1]
	}
	return lines
}

func isVisualPaddingLine(s string) bool {
	for _, r := range s {
		if r != '\u2800' && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func renderColoredFragments(frag string) (plain string, ansi string) {
	var pb, ab strings.Builder
	var usedColor bool
	i := 0
	for i < len(frag) {
		loc := reSpanOpen.FindStringSubmatchIndex(frag[i:])
		if loc == nil {
			t := stripTags(frag[i:])
			pb.WriteString(t)
			ab.WriteString(t)
			break
		}
		if loc[0] > 0 {
			t := stripTags(frag[i : i+loc[0]])
			pb.WriteString(t)
			ab.WriteString(t)
		}
		startContent := i + loc[1]
		colorHex := frag[i+loc[2] : i+loc[3]]
		closeIdx := strings.Index(frag[startContent:], "</span>")
		if closeIdx < 0 {
			t := stripTags(frag[startContent:])
			pb.WriteString(t)
			ab.WriteString(t)
			break
		}
		text := frag[startContent : startContent+closeIdx]
		if text != "" {
			r, g, b := enhanceLogoRGB(parseHexRGB(colorHex))
			pb.WriteString(text)
			ab.WriteString(termcolor.ForegroundRGB(r, g, b))
			ab.WriteString(text)
			usedColor = true
		}
		i = startContent + closeIdx + len("</span>")
	}
	out := ab.String()
	if usedColor {
		return pb.String(), out + termcolor.Reset
	}
	return pb.String(), out
}

func stripTags(s string) string {
	return reAnyTag.ReplaceAllString(s, "")
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
