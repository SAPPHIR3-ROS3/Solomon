//go:build !windows

package main

import (
	"os/exec"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon/server/detach"
)

func configureDesktopProcess(cmd *exec.Cmd) {
	detach.Configure(cmd)
}
