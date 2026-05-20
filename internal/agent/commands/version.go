package commands

import (
	"fmt"
	"io"
	"runtime/debug"
)

func VersionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return v
}

func WriteVersion(w io.Writer) {
	fmt.Fprintln(w, VersionString())
}

func Version(d Deps) error {
	WriteVersion(d.Out)
	return nil
}
