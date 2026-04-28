package commands

import "fmt"

func Plan(d Deps) error {
	d.SetMode("plan")
	fmt.Fprintln(d.Out, "Mode: plan")
	return nil
}
