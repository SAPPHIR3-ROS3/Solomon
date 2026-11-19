package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"orbit", "laboratory"} {
		path := name + suffix
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
