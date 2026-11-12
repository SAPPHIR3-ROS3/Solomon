package shellpolicy

import (
	"fmt"
	"strings"
	"unicode"
)

func commands(text string, depth int) ([][]string, error) {
	var result [][]string
	var words []string
	var word strings.Builder
	started := false
	var quote byte
	flushWord := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	flushCommand := func() {
		flushWord()
		if len(words) > 0 {
			result = append(result, words)
			words = nil
		}
	}
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if quote == '\'' {
			if ch == quote {
				quote = 0
			} else {
				word.WriteByte(ch)
			}
			continue
		}
		if ch == '\\' && i+1 < len(text) {
			next := text[i+1]
			if quote == 0 || strings.ContainsRune("$`\"\\\n", rune(next)) {
				if next != '\n' {
					word.WriteByte(next)
					started = true
				}
				i++
				continue
			}
		}
		if ch == '`' {
			return nil, fmt.Errorf("backtick shell substitutions are not allowed; use a literal command or Go/SDK operations")
		}
		if ch == '$' && i+1 < len(text) && text[i+1] == '(' {
			end, err := substitutionEnd(text, i+2)
			if err != nil {
				return nil, err
			}
			if i+2 < len(text) && text[i+2] == '(' {
				if !simpleArithmetic.MatchString(text[i+3 : end-1]) {
					return nil, fmt.Errorf("only simple shell counters are allowed; perform other calculations in Go")
				}
			} else if err := validate(text[i+2:end], depth+1); err != nil {
				return nil, err
			}
			word.WriteString(substitutionValue)
			started = true
			i = end
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				word.WriteByte(ch)
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			started = true
			continue
		}
		if ch == '#' && !started {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			flushCommand()
			continue
		}
		if ch == '<' && i+1 < len(text) && text[i+1] == '<' {
			return nil, fmt.Errorf("shell heredocs are not allowed in orchestration; use Go and SDK file helpers")
		}
		if ch == '&' && started && (strings.HasSuffix(word.String(), ">") || strings.HasSuffix(word.String(), "<")) {
			word.WriteByte(ch)
			continue
		}
		if strings.ContainsRune(";|&\n(){}", rune(ch)) {
			flushCommand()
			continue
		}
		if unicode.IsSpace(rune(ch)) {
			flushWord()
			continue
		}
		word.WriteByte(ch)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated shell quote")
	}
	flushCommand()
	return result, nil
}
