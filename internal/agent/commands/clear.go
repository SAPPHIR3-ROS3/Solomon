package commands

import "fmt"

func Clear(d Deps) error {
	fmt.Fprint(d.Out, "\033[2J\033[H")
	return nil
}
