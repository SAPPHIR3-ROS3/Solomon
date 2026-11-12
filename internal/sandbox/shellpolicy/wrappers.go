package shellpolicy

import (
	"fmt"
	"strings"
)

func validateLauncher(name string, args []string, depth int) (bool, error) {
	switch name {
	case "alias", "unalias", "source", ".", "function":
		return true, rejected(name)
	case "xargs":
		words, err := launcherArgs(args, []string{"-I", "-n", "-P", "-L", "-d", "-E", "--replace", "--max-args", "--max-procs", "--max-lines", "--delimiter", "--eof"})
		if err != nil {
			return true, err
		}
		return true, validateWords(words, depth+1)
	case "find":
		for i, arg := range args {
			if contains([]string{"-exec", "-execdir", "-ok", "-okdir"}, arg) {
				end := i + 1
				for end < len(args) && args[end] != ";" && args[end] != "+" {
					end++
				}
				if err := validateWords(args[i+1:end], depth+1); err != nil {
					return true, err
				}
			}
		}
		return true, nil
	case "docker", "podman":
		words, err := launcherArgs(args, []string{"-H", "--host", "--context", "--config", "-c"})
		if err != nil {
			return true, err
		}
		if len(words) > 0 && words[0] == "container" {
			words = words[1:]
		}
		if len(words) > 0 && words[0] == "compose" {
			words, err = launcherArgs(words[1:], []string{"-f", "--file", "-p", "--project-name", "--profile", "--env-file"})
			if err != nil {
				return true, err
			}
		}
		if len(words) == 0 || !contains([]string{"exec", "run"}, words[0]) {
			return true, nil
		}
		entrypoint := ""
		for i, option := range words[1:] {
			if option == "--entrypoint" && i+2 < len(words) {
				entrypoint = words[i+2]
			}
			if strings.HasPrefix(option, "--entrypoint=") {
				entrypoint = strings.TrimPrefix(option, "--entrypoint=")
			}
		}
		words, err = launcherArgs(words[1:], []string{"-e", "--env", "--env-file", "-u", "--user", "-w", "--workdir", "--name", "--entrypoint", "-v", "--volume", "--mount", "--network", "--label", "-l", "-p", "--publish", "--hostname", "--add-host"})
		if err != nil {
			return true, err
		}
		if len(words) == 0 {
			return true, nil
		}
		if entrypoint != "" {
			return true, validateWords(append([]string{entrypoint}, words[1:]...), depth+1)
		}
		return true, validateWords(words[1:], depth+1)
	case "kubectl":
		values := []string{"-n", "--namespace", "--context", "--cluster", "--kubeconfig", "--user", "--server", "-c", "--container"}
		words, err := launcherArgs(args, values)
		if err != nil {
			return true, err
		}
		if len(words) == 0 || words[0] != "exec" {
			return true, nil
		}
		words, err = launcherArgs(words[1:], values)
		if err != nil {
			return true, err
		}
		if len(words) > 0 {
			words = words[1:]
		}
		if len(words) > 0 && words[0] == "--" {
			words = words[1:]
		}
		return true, validateWords(words, depth+1)
	}
	return false, nil
}

func launcherArgs(args []string, values []string) ([]string, error) {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		option := args[0]
		args = args[1:]
		if option == "--" {
			return args, nil
		}
		if strings.Contains(option, "=") {
			continue
		}
		if contains(values, option) {
			if len(args) == 0 {
				return nil, fmt.Errorf("shell wrapper option %s requires a value", option)
			}
			args = args[1:]
			continue
		}
		if contains([]string{"-r", "-0", "-t", "-i", "-it", "-ti", "-d", "--rm", "--interactive", "--tty", "--detach", "--no-run-if-empty", "--null", "--verbose", "--privileged", "--no-TTY"}, option) {
			continue
		}
		return nil, fmt.Errorf("unsupported shell wrapper option %s; use a literal project command without indirect execution", option)
	}
	return args, nil
}

func validateEnvSplit(args []string, depth int) (bool, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-S" || arg == "--split-string" {
			return true, validate(strings.Join(args[i+1:], " "), depth+1)
		}
		if strings.HasPrefix(arg, "--split-string=") {
			return true, validate(strings.TrimPrefix(arg, "--split-string=")+" "+strings.Join(args[i+1:], " "), depth+1)
		}
		if contains([]string{"-u", "-C", "--unset", "--chdir"}, arg) {
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") && !assignment.MatchString(arg) {
			break
		}
	}
	return false, nil
}
