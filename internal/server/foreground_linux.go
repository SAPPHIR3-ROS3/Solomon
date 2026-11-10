package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func (p *unixTerminalProcess) Foreground() (bool, string) {
	group, err := unix.IoctlGetInt(int(p.file.Fd()), unix.TIOCGPGRP)
	if err != nil || group <= 0 || p.cmd.Process == nil {
		return false, filepath.Base(p.cmd.Path)
	}
	name := func(pid int) string {
		path, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
		if err == nil {
			return filepath.Base(path)
		}
		data, _ := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
		return strings.TrimSpace(string(data))
	}
	title := name(group)
	if title == "" {
		title = filepath.Base(p.cmd.Path)
	}
	running := group != p.cmd.Process.Pid
	if !running {
		executable, executableErr := os.Readlink("/proc/" + strconv.Itoa(group) + "/exe")
		shell, shellErr := filepath.EvalSymlinks(p.cmd.Path)
		running = executableErr == nil && shellErr == nil && executable != shell
	}
	return running, title
}
