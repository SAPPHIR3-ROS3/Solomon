package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type PromptIO struct {
	Stdin    io.Reader
	Out      io.Writer
	ReadLine func(prompt string) (string, error)
}

func (p PromptIO) promptOut() io.Writer {
	if p.Out != nil {
		return p.Out
	}
	return os.Stdout
}

func (p PromptIO) promptIn() io.Reader {
	if p.Stdin != nil {
		return p.Stdin
	}
	return os.Stdin
}

func ReadPromptLine(p PromptIO, prompt string) (string, error) {
	if p.ReadLine != nil {
		line, err := p.ReadLine(prompt)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	out := p.promptOut()
	if prompt != "" {
		fmt.Fprint(out, prompt)
	}
	br := bufio.NewScanner(p.promptIn())
	if !br.Scan() {
		if err := br.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(br.Text()), nil
}

