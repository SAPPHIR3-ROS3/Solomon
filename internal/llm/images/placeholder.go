package images

import (
	"bytes"
	"fmt"
	"regexp"
)

var PlaceholderRE = regexp.MustCompile(`\[img-(\d+)\]`)

var pngMagic = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func Placeholder(seq int) string {
	return fmt.Sprintf("[img-%d]", seq)
}

func Atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func MIMEForBinary(data []byte) (mime string, ok bool) {
	if len(data) < 3 {
		return "", false
	}
	switch {
	case len(data) >= len(pngMagic) && bytes.Equal(data[:len(pngMagic)], pngMagic):
		return "image/png", true
	case data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg", true
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif", true
	default:
		return "", false
	}
}
