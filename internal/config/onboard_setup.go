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

func RunInitialSetup(pio PromptIO, errOut io.Writer, cfg *Root, configExists bool) error {
	out := pio.promptOut()
	if errOut == nil {
		errOut = os.Stderr
	}
	if !NeedsOnboard(cfg) {
		return nil
	}
	if !configExists {
		fmt.Fprintln(out, "Welcome to Solomon. Set up your LLM provider to get started.")
		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "LLM setup incomplete. Let's finish configuration.")
		fmt.Fprintln(out)
	}
	opts := OnboardOpts{RequireProvider: true}
	for NeedsOnboard(cfg) {
		res, err := RunOnboardWizard(pio, cfg, opts)
		if err != nil {
			fmt.Fprintf(errOut, "%v\n", err)
			if strings.Contains(err.Error(), "unexpected end of input") {
				return err
			}
			fmt.Fprintln(out, "Setup failed. Please try again.")
			fmt.Fprintln(out)
			continue
		}
		ApplyOnboardMerge(cfg, res)
		if NeedsOnboard(cfg) {
			fmt.Fprintln(out, "Provider, API key, and model are required.")
			fmt.Fprintln(out)
		}
	}
	return Save(cfg)
}
