package test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	root := repositoryRootForTestLayout(t)
	for _, argument := range []string{"--help", "-h", "help"} {
		t.Run(argument, func(t *testing.T) {
			command := exec.Command("go", "run", "./cmd/solomon", argument)
			command.Dir = root
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("solomon %s failed: %v\n%s", argument, err, output)
			}
			for _, expected := range []string{
				"Usage:",
				"solomon exec [options] <prompt>",
				"solomon temp exec [options] <prompt>",
				"solomon server <command>",
				"-h, --help",
				"/help",
			} {
				if !strings.Contains(string(output), expected) {
					t.Fatalf("solomon %s help missing %q: %s", argument, expected, output)
				}
			}
		})
	}
}
