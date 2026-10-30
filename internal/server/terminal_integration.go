package server

import (
	"os"
	"path/filepath"
)

func terminalIntegration(shell string, args, env []string) ([]string, []string, func(), error) {
	name := filepath.Base(shell)
	if name != "bash" && name != "zsh" {
		return args, env, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "solomon-shell-")
	if err != nil {
		return nil, nil, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	var content, file string
	if name == "bash" {
		file = filepath.Join(dir, "bashrc")
		content = `[[ -f ~/.bashrc ]] && source ~/.bashrc
__solomon_prompt() { printf '\033]133;A\007'; }
PROMPT_COMMAND="__solomon_prompt${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
`
		args = []string{"--rcfile", file, "-i"}
	} else {
		file = filepath.Join(dir, ".zshrc")
		original := os.Getenv("ZDOTDIR")
		if original == "" {
			original = os.Getenv("HOME")
		}
		env = append(env, "SOLOMON_ORIGINAL_ZDOTDIR="+original, "ZDOTDIR="+dir)
		content = `ZDOTDIR=$SOLOMON_ORIGINAL_ZDOTDIR
[[ -f "$ZDOTDIR/.zshrc" ]] && source "$ZDOTDIR/.zshrc"
autoload -Uz add-zsh-hook
__solomon_prompt() { printf '\033]133;A\007'; }
__solomon_preexec() { printf '\033]133;C\007'; }
add-zsh-hook precmd __solomon_prompt
add-zsh-hook preexec __solomon_preexec
`
	}
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	return args, env, cleanup, nil
}
