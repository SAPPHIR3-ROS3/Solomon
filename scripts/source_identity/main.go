// source_identity fingerprints source inputs before compilation and version stamping.
package main

import (
	"fmt"
	"os"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func main() {
	tree, err := updater.SnapshotSourceTree(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(tree)
}
