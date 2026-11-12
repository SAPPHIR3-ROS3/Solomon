package shellpolicy

import (
	"fmt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/validation"
	"regexp"
	"strings"
)

const Guidance = "Orchestration logic must be Go only: use Go standard-library packages and SDK helpers for parsing, JSON, filtering, calculations, and file transformations. Never execute inline Python, JavaScript/TypeScript, Ruby, Perl, Lua, PHP, awk, jq programs, or nested interpreter snippets through Shell. Do not move generated helper code into temporary files, encoded payloads, or project scripts to bypass this rule. Shell is only for existing host commands and existing project test/build entrypoints. Target-language source may be written as project file content when the task requires it; it is data, not an orchestration helper."

var assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*=`)
var leadingRedirect = regexp.MustCompile(`^[0-9]*(?:[<>]&[0-9-]+|<>[^<>]*|>>?[^<>]*|<[^<>]*)$`)

var interpreterName = regexp.MustCompile(`^(py|python(?:\d+(?:\.\d+)*)?|pypy\d*|node(?:js)?|bun|deno|tsx|ts-node|ruby\d*(?:\.\d+)*|perl\d*|php\d*(?:\.\d+)*|lua\d*(?:\.\d+)*|luajit|[gnm]?awk|jq|pwsh|powershell)$`)

func Validate(command string) error {
	if err := validate(command, 0); err != nil {
		return &validation.Issue{Kind: "policy", Component: "shell", Field: "command", Problem: err.Error(), Cause: err}
	}
	return nil
}

func validate(command string, depth int) error {
	if depth > 8 {
		return fmt.Errorf("shell wrapper nesting is not allowed in orchestration; use Go directly")
	}
	groups, err := commands(command, depth)
	if err != nil {
		return err
	}
	for _, words := range groups {
		if err := validateWords(words, depth); err != nil {
			return err
		}
	}
	return nil
}

func validateWords(words []string, depth int) error {
	if depth > 8 {
		return fmt.Errorf("shell wrapper nesting is not allowed in orchestration; use Go directly")
	}
	for len(words) > 0 {
		name := executableName(words[0])
		args := words[1:]
		if leadingRedirect.MatchString(words[0]) {
			if strings.HasSuffix(words[0], ">") || strings.HasSuffix(words[0], "<") {
				if len(args) == 0 {
					return fmt.Errorf("shell redirection requires a target")
				}
				words = args[1:]
			} else {
				words = args
			}
			continue
		}
		if handled, err := validateLauncher(name, args, depth); handled {
			return err
		}
		if name == "eval" {
			return rejected(name)
		}
		if assignment.MatchString(words[0]) || contains([]string{"if", "then", "else", "elif", "do", "!", "time"}, name) {
			words = args
			continue
		}
		if name == "env" {
			if handled, err := validateEnvSplit(args, depth); handled {
				return err
			}
		}
		if contains([]string{"exec", "command", "builtin", "env", "sudo", "timeout", "nohup", "nice"}, name) {
			if name == "command" && len(args) > 0 && contains([]string{"-v", "-V"}, args[0]) {
				return nil
			}
			words = unwrap(name, args)
			continue
		}
		if strings.ContainsAny(words[0], "$%") || strings.Contains(words[0], substitutionValue) {
			return fmt.Errorf("dynamic shell executables are not allowed in orchestration; use literal host commands and Go for processing")
		}
		if contains([]string{"bash", "sh", "zsh", "dash", "ksh", "fish", "cmd"}, name) {
			for _, arg := range args {
				if arg == "/c" || (strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "c")) {
					return rejected(name)
				}
			}
		}
		if name == "ssh" {
			for i, option := range args {
				value := ""
				if option == "-o" && i+1 < len(args) {
					value = args[i+1]
				}
				if strings.HasPrefix(option, "-o") && len(option) > 2 {
					value = option[2:]
				}
				key, command, ok := strings.Cut(value, "=")
				if ok && contains([]string{"proxycommand", "localcommand", "remotecommand"}, strings.ToLower(key)) {
					if err := validate(command, depth+1); err != nil {
						return err
					}
				}
			}
			for i := 0; i < len(args); i++ {
				if strings.HasPrefix(args[i], "-") {
					if contains([]string{"-o", "-p", "-i", "-l", "-J", "-F"}, args[i]) {
						i++
					}
					continue
				}
				return validate(strings.Join(args[i+1:], " "), depth+1)
			}
		}
		if name == "uv" && len(args) > 0 && args[0] == "run" {
			return validateWords(args[1:], depth+1)
		}
		if contains([]string{"npx", "uvx"}, name) {
			for len(args) > 0 && contains([]string{"-y", "--yes", "--"}, args[0]) {
				args = args[1:]
			}
			return validateWords(args, depth+1)
		}
		if contains([]string{"npm", "pnpm", "yarn"}, name) && len(args) > 0 && args[0] == "exec" {
			args = args[1:]
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			return validateWords(args, depth+1)
		}
		if interpreterName.MatchString(name) && inlineProgram(name, args) {
			return rejected(name)
		}
		return nil
	}
	return nil
}

func executableName(word string) string {
	if index := strings.LastIndexAny(word, "/\\"); index >= 0 {
		word = word[index+1:]
	}
	return strings.TrimSuffix(strings.ToLower(word), ".exe")
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func rejected(name string) error {
	return fmt.Errorf("inline %s programs are not allowed in orchestrate shell calls; perform parsing, calculations, filtering, and transformations in Go with SDK helpers; do not move the program into a temporary script", name)
}

func unwrap(name string, args []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		option := args[0]
		args = args[1:]
		takesValue := false
		switch name {
		case "sudo":
			takesValue = contains([]string{"-u", "-g", "-C", "-h", "-p", "--user", "--group", "--host", "--prompt"}, option)
		case "env":
			takesValue = contains([]string{"-u", "-C", "--unset", "--chdir"}, option)
		case "timeout":
			takesValue = contains([]string{"-s", "-k", "--signal", "--kill-after"}, option)
		case "nice":
			takesValue = option == "-n" || option == "--adjustment"
		case "exec":
			takesValue = option == "-a"
		}
		if takesValue && len(args) > 0 {
			args = args[1:]
		}
	}
	if name == "timeout" && len(args) > 0 {
		args = args[1:]
	}
	return args
}
