package tokcount

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pkoukk/tiktoken-go"
)

const (
	DefaultEncoding = tiktoken.MODEL_O200K_BASE
	DefaultModel    = "gpt-4o"
)

var (
	encOnce sync.Once
	enc     *tiktoken.Tiktoken
	encErr  error
)

func defaultEncoder() (*tiktoken.Tiktoken, error) {
	encOnce.Do(func() {
		enc, encErr = tiktoken.GetEncoding(DefaultEncoding)
	})
	return enc, encErr
}

func EncoderForModel(model string) (*tiktoken.Tiktoken, error) {
	m := strings.TrimSpace(model)
	if m == "" {
		return defaultEncoder()
	}
	tkm, err := tiktoken.EncodingForModel(m)
	if err != nil {
		return defaultEncoder()
	}
	return tkm, nil
}

func TextTokens(text string, model string) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	tkm, err := EncoderForModel(model)
	if err != nil {
		return roughTextTokens(text)
	}
	return int64(len(tkm.Encode(text, nil, nil)))
}

func roughTextTokens(text string) int64 {
	n := utf8.RuneCountInString(text)
	if n <= 0 {
		return 0
	}
	return int64((n + 2) / 3)
}
