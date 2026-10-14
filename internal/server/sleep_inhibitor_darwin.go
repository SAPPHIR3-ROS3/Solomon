package server

import (
	"os"
	"os/exec"
	"strconv"
)

func acquireSleepInhibitor() (func(), error) {
	command := exec.Command("/usr/bin/caffeinate", "-i", "-s", "-w", strconv.Itoa(os.Getpid()))
	if err := command.Start(); err != nil {
		return nil, err
	}
	go func() { _ = command.Wait() }()
	return func() { _ = command.Process.Kill() }, nil
}
