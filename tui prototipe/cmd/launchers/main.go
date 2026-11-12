package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"orbit", "mission"} {
		path := name + suffix
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		target := "laboratory" + suffix
		if err := os.Symlink(target, path); err == nil {
			continue
		}
		source, err := os.Open(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		destination, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0755)
		if err != nil {
			source.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_, copyErr := io.Copy(destination, source)
		closeErr := destination.Close()
		source.Close()
		if copyErr != nil {
			fmt.Fprintln(os.Stderr, copyErr)
			os.Exit(1)
		}
		if closeErr != nil {
			fmt.Fprintln(os.Stderr, closeErr)
			os.Exit(1)
		}
		if _, err = filepath.Abs(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
