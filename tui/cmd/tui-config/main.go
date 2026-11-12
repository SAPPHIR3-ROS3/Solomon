package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/tui/live"
)

func main() {
	check := flag.Bool("check", false, "Validate the current TUI interface configuration")
	printDefault := flag.Bool("print", false, "Print the complete default TUI configuration")
	flag.Parse()
	if *printDefault {
		data, err := json.MarshalIndent(live.Default(), "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}
	path, err := live.Path()
	if err == nil {
		if *check {
			_, err = live.Read(path)
		} else {
			err = live.Export(path)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(path)
}
