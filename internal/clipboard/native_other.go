//go:build !linux

package clipboard

import "context"

func readLinuxImage(context.Context) ([]byte, error) { return nil, ErrUnavailable }
func linuxImageStatus() error                        { return ErrUnavailable }
func pasteImageLinux(string) error                   { return ErrUnavailable }
