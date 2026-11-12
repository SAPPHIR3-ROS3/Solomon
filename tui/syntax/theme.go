package syntax

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

//go:embed cursor-dark.json
var rawTheme []byte

type rule struct {
	Scope    any
	Settings struct{ Foreground string }
}
type theme struct {
	Colors      map[string]string
	TokenColors []rule   `json:"tokenColors"`
	Brackets    []string `json:"solomonBracketColors"`
}

var shared = func() theme {
	var data theme
	if err := json.Unmarshal(rawTheme, &data); err != nil {
		panic(err)
	}
	return data
}()

func Color(scope string) string {
	value := shared.Colors["editor.foreground"]
	best := -1
	for _, item := range shared.TokenColors {
		if item.Settings.Foreground == "" {
			continue
		}
		scopes := []string{}
		switch v := item.Scope.(type) {
		case string:
			scopes = strings.Split(v, ",")
		case []any:
			for _, s := range v {
				if text, ok := s.(string); ok {
					scopes = append(scopes, text)
				}
			}
		}
		for _, candidate := range scopes {
			candidate = strings.TrimSpace(candidate)
			if scope == candidate || strings.HasPrefix(scope, candidate+".") {
				if len(candidate) >= best {
					best = len(candidate)
					value = item.Settings.Foreground
				}
			}
		}
	}
	return opaque(value)
}
func Foreground() string       { return opaque(shared.Colors["editor.foreground"]) }
func Bracket(depth int) string { return shared.Brackets[depth%len(shared.Brackets)] }
func ANSI(value, color string) string {
	if value == "" {
		return ""
	}
	rgb, err := strconv.ParseUint(strings.TrimPrefix(opaque(color), "#"), 16, 24)
	if err != nil {
		return value
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", rgb>>16, (rgb>>8)&255, rgb&255, value)
}
func opaque(color string) string {
	if len(color) != 9 {
		return color
	}
	rgba, err := strconv.ParseUint(color[1:], 16, 32)
	if err != nil {
		return Foreground()
	}
	background, _ := strconv.ParseUint(strings.TrimPrefix(shared.Colors["editor.background"], "#"), 16, 24)
	alpha := int(rgba & 255)
	blend := func(fg, bg uint64) int { return (int(fg)*alpha + int(bg)*(255-alpha) + 127) / 255 }
	return fmt.Sprintf("#%02x%02x%02x", blend((rgba>>24)&255, (background>>16)&255), blend((rgba>>16)&255, (background>>8)&255), blend((rgba>>8)&255, background&255))
}
