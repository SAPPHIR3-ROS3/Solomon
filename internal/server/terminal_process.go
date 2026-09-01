package server

import "io"

type terminalProcess interface {
	io.ReadWriteCloser
	Kill() error
	Resize(cols, rows uint16) error
	Wait() error
}

type terminalProcessOptions struct {
	Args  []string
	Cols  uint16
	Cwd   string
	Env   []string
	Rows  uint16
	Shell string
}
