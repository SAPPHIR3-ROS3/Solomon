package config

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func RunInitialSetup(stdin io.Reader, out, errOut io.Writer, cfg *Root, configExists bool) error {
	if stdin == nil {
		stdin = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
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
		res, err := RunOnboardWizard(stdin, out, cfg, opts)
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
