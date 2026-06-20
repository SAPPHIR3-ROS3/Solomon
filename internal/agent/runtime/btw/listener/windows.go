//go:build windows

package listener

import "time"

func ready(fd int, d time.Duration) bool {
	return false
}
