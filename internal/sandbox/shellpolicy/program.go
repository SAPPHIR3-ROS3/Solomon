package shellpolicy

import "strings"

func inlineProgram(name string, args []string) bool {
	if len(args) == 0 {
		return true
	}
	if contains([]string{"awk", "gawk", "nawk", "mawk", "jq"}, name) {
		return !contains([]string{"--version", "--help", "-V"}, args[0])
	}
	testRunner := false
	python := name == "py" || strings.HasPrefix(name, "python") || strings.HasPrefix(name, "pypy")
	node := contains([]string{"node", "nodejs", "bun", "tsx", "ts-node"}, name)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		if arg == "--" {
			return i+1 >= len(args) || stdinProgram(args[i+1])
		}
		if stdinProgram(arg) {
			return true
		}
		if name == "deno" && arg == "eval" {
			return true
		}
		if (name == "deno" || name == "bun") && arg == "run" {
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			return false
		}
		if contains([]string{"--version", "--help", "-V", "-v", "-h"}, arg) {
			if len(args) == 1 {
				return false
			}
			testRunner = true
			continue
		}
		if node && arg == "--test" {
			testRunner = true
			continue
		}
		if strings.HasPrefix(lower, "--eval") || strings.HasPrefix(lower, "--print") || strings.HasPrefix(lower, "--command") || strings.HasPrefix(lower, "--encodedcommand") {
			return true
		}
		if python {
			if arg == "-m" {
				return i+1 >= len(args) || !approvedPythonModule(args[i+1])
			}
			if strings.HasPrefix(arg, "-m") {
				return !approvedPythonModule(arg[2:])
			}
			if arg == "-W" || arg == "-X" {
				i++
				if i >= len(args) {
					return true
				}
				continue
			}
			if strings.HasPrefix(arg, "-W") || strings.HasPrefix(arg, "-X") {
				continue
			}
			if !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "c") {
				return true
			}
		}
		if node {
			if strings.HasPrefix(arg, "-r") && len(arg) > 2 {
				if encodedProgram(arg[2:]) {
					return true
				}
				continue
			}
			if contains([]string{"-r", "--require", "--import", "--loader", "--experimental-loader", "--input-type"}, arg) {
				i++
				if i >= len(args) || encodedProgram(args[i]) {
					return true
				}
				continue
			}
			if strings.HasPrefix(arg, "--require=") || strings.HasPrefix(arg, "--import=") || strings.HasPrefix(arg, "--loader=") {
				_, value, _ := strings.Cut(arg, "=")
				if encodedProgram(value) {
					return true
				}
				continue
			}
		}
		if name == "powershell" || name == "pwsh" {
			if lower == "-file" {
				return i+1 >= len(args) || stdinProgram(args[i+1])
			}
			if contains([]string{"-noprofile", "-noninteractive", "-noexit", "-nologo"}, lower) {
				continue
			}
			return true
		}
		if strings.HasPrefix(name, "php") && strings.HasPrefix(lower, "--run") {
			return true
		}
		if !strings.HasPrefix(arg, "--") {
			flags := arg[1:]
			if strings.Contains(flags, "e") || (strings.HasPrefix(name, "php") && strings.Contains(flags, "r")) || (node && strings.Contains(flags, "p")) {
				return true
			}
		}
	}
	return !testRunner
}

func approvedPythonModule(module string) bool {
	return contains([]string{"pytest", "unittest", "compileall", "pip", "venv", "build"}, module)
}

func stdinProgram(arg string) bool {
	return contains([]string{"-", "/dev/stdin", "/dev/fd/0"}, arg) || strings.HasPrefix(arg, "<")
}

func encodedProgram(arg string) bool {
	return strings.HasPrefix(strings.ToLower(arg), "data:") || strings.HasPrefix(strings.ToLower(arg), "javascript:") || strings.ContainsAny(arg, "\n\r")
}
