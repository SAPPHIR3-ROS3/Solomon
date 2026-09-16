// npm_deps verifies and, when necessary, installs a project's locked npm dependencies.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/npm_deps <directory>")
		os.Exit(2)
	}

	dir, err := filepath.Abs(os.Args[1])
	if err != nil {
		fail("resolve directory: %v", err)
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			fail("%s is missing in %s", name, dir)
		}
	}

	npm, err := exec.LookPath("npm")
	if err != nil {
		fail("npm not found in PATH; install Node.js and npm before continuing")
	}

	if npmDependenciesReady(npm, dir) {
		fmt.Printf("npm dependencies ready: %s\n", dir)
		return
	}

	fmt.Printf("npm dependencies missing or incomplete: %s\n", dir)
	fmt.Println("  → installing from package-lock.json")
	cmd := exec.Command(npm, "ci", "--no-audit", "--no-fund")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail("npm ci failed in %s: %v", dir, err)
	}
	fmt.Printf("npm dependencies ready: %s\n", dir)
}

func npmDependenciesReady(npm, dir string) bool {
	cmd := exec.Command(npm, "ls", "--depth=0", "--silent")
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func fail(format string, values ...any) {
	fmt.Fprintf(os.Stderr, "npm-deps: "+format+"\n", values...)
	os.Exit(1)
}
