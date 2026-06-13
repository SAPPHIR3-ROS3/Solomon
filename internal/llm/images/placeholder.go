package images

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images/token"

func Placeholder(seq int) string {
	return token.VisibleTag(seq)
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
