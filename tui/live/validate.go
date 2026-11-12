package live

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var colorPattern = regexp.MustCompile("^#[0-9a-fA-F]{6}$")

func (c Config) Validate() error {
	if c.Version != Version {
		return fmt.Errorf("unsupported TUI configuration version %d", c.Version)
	}
	defaults := Default()
	if len(c.Colors) != len(defaults.Colors) {
		return fmt.Errorf("TUI color roles do not match the supported schema")
	}
	for role, value := range c.Colors {
		if _, ok := defaults.Colors[role]; !ok {
			return fmt.Errorf("unknown color role %q", role)
		}
		if !colorPattern.MatchString(value) {
			return fmt.Errorf("invalid RGB color for %s: %q", role, value)
		}
	}
	l := c.Layout
	if l.HomeWidth < 36 || l.HomeWidth > 200 || l.ComposerRows < 1 || l.ComposerRows > 8 || l.MaxInputRows < 1 || l.MaxInputRows > 12 || l.SuggestionRows < 1 || l.SuggestionRows > 12 || l.Margin < 0 || l.Margin > 8 {
		return fmt.Errorf("TUI layout dimensions are outside supported bounds")
	}
	if l.Border != "rounded" && l.Border != "square" && l.Border != "heavy" {
		return fmt.Errorf("unknown TUI border %q", l.Border)
	}
	if len(c.Text) != len(defaults.Text) {
		return fmt.Errorf("TUI text roles do not match the supported schema")
	}
	for key, value := range c.Text {
		if _, ok := defaults.Text[key]; !ok {
			return fmt.Errorf("unknown text role %q", key)
		}
		if len([]rune(value)) > 256 || strings.TrimSpace(value) == "" {
			return fmt.Errorf("invalid text length for %s", key)
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return fmt.Errorf("control character in %s", key)
			}
		}
	}
	return c.validateKeys()
}
