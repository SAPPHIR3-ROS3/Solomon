// install_path resolves Go's install directory, including multiple GOPATH entries.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	output, err := exec.Command("go", "env", "GOBIN", "GOPATH").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lines := strings.Split(strings.TrimRight(string(output), "\r\n"), "\n")
	if len(lines) != 2 {
		fmt.Fprintln(os.Stderr, "go env did not return an install directory")
		os.Exit(1)
	}
	bin := strings.TrimSpace(lines[0])
	if bin == "" {
		paths := filepath.SplitList(strings.TrimSpace(lines[1]))
		if len(paths) == 0 {
			fmt.Fprintln(os.Stderr, "empty GOPATH")
			os.Exit(1)
		}
		bin = filepath.Join(paths[0], "bin")
	}
	fmt.Println(bin)
}
