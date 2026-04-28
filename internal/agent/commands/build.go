package commands

import "fmt"

func Build(d Deps) error {
	d.SetMode("build")
	fmt.Fprintln(d.Out, "Mode: build")
	return nil
}
