//go:build !linux && !darwin && !windows

package server

import "errors"

func acquireSleepInhibitor() (func(), error) {
	return nil, errors.New("preventing sleep is currently supported on Linux, macOS and Windows")
}
