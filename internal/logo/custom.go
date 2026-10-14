package logo

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/charmbracelet/x/ansi"
)

const (
	maxLogoBytes   = 64 * 1024
	maxColorBytes  = 256 * 1024
	maxLogoRows    = 64
	maxLogoColumns = 160
)

// Only the welcome banner uses these overrides; application icons stay embedded.
func welcomeSource() (string, string) {
	home, err := paths.SolomonHome()
	if err != nil {
		return ASCII, ColorsRaw
	}
	asciiDir := filepath.Join(home, "banner", "ascii")
	colorsDir := filepath.Join(home, "banner", "colors")
	for _, dir := range []string{asciiDir, colorsDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return ASCII, ColorsRaw
		}
	}
	raw, err := readLogoFile(filepath.Join(asciiDir, "logo.txt"), maxLogoBytes)
	if err != nil {
		return ASCII, ColorsRaw
	}
	rows, ok := validLogoRows(raw)
	if !ok {
		return ASCII, ColorsRaw
	}
	colors, err := readLogoFile(filepath.Join(colorsDir, "colors.txt"), maxColorBytes)
	if os.IsNotExist(err) {
		return raw, ""
	}
	if err != nil || !validLogoColors(colors, rows) {
		return ASCII, ColorsRaw
	}
	return raw, colors
}

func readLogoFile(path string, limit int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return "", fmt.Errorf("invalid logo file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > limit {
		return "", fmt.Errorf("logo file too large")
	}
	return string(data), nil
}

func logoRows(raw string) []string {
	return strings.Split(strings.TrimRight(strings.ReplaceAll(raw, "\r\n", "\n"), "\n"), "\n")
}

func validLogoRows(raw string) ([]string, bool) {
	if !utf8.ValidString(raw) || len(raw) > maxLogoBytes {
		return nil, false
	}
	rows := logoRows(raw)
	if len(rows) > maxLogoRows {
		return nil, false
	}
	visible := false
	for _, row := range rows {
		if ansi.StringWidth(row) > maxLogoColumns {
			return nil, false
		}
		for _, r := range row {
			if unicode.IsControl(r) || (unicode.Is(unicode.Cf, r) && r != '\u200d') || r == '\u2028' || r == '\u2029' {
				return nil, false
			}
			if !unicode.IsSpace(r) && r != '\u2800' && !unicode.IsMark(r) && r != '\u200d' {
				visible = true
			}
		}
	}
	return rows, visible
}

// Each row contains one six-digit RGB value per Unicode code point, including spaces.
func validLogoColors(raw string, rows []string) bool {
	colors := logoRows(raw)
	if len(colors) != len(rows) {
		return false
	}
	for i, row := range rows {
		fields := strings.Fields(colors[i])
		if len(fields) != utf8.RuneCountInString(row) {
			return false
		}
		for _, field := range fields {
			if len(field) != 6 {
				return false
			}
			if _, err := hex.DecodeString(field); err != nil {
				return false
			}
		}
	}
	return true
}
