//go:build !windows

package server

func containViteTree() (func(), error) { return func() {}, nil }
