package main

import (
	"fmt"
	"io"
)

func cliHelpRequested(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func writeCLIHelp(w io.Writer) {
	fmt.Fprintln(w, `Solomon — AI coding assistant

Usage:
  solomon [directory]
  solomon tui [directory]
  solomon attach [directory]
  solomon exec [options] <prompt>
  solomon temp exec [options] <prompt>
  solomon server <command>

Commands:
  version                         Print the installed version
  upgrade                         Install the latest release
  init                            Create the default configuration
  add                             Install a skill
  remove skill <name>             Remove a skill
  server start|status|stop|...    Manage the local server

Options:
  -h, --help                      Show this help

Inside the REPL, use /help for the complete slash-command list.`)
}
