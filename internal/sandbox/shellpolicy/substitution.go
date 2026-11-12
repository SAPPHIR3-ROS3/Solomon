package shellpolicy

import (
	"fmt"
	"regexp"
)

var simpleArithmetic = regexp.MustCompile(`^[A-Za-z_0-9 +*/%<>=!&|^~?:(),\t\r\n-]+$`)

const substitutionValue = "__solomon_command_value__"

func substitutionEnd(text string, start int) (int, error) {
	level := 1
	var quote byte
	for i := start; i < len(text); i++ {
		ch := text[i]
		if ch == '\\' && quote != '\'' {
			i++
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			quote = ch
			continue
		}
		if ch == '(' {
			level++
		}
		if ch == ')' {
			level--
			if level == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated shell command substitution")
}
