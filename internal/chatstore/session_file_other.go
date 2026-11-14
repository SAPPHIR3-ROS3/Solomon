//go:build !windows

package chatstore

func retrySessionRename(error) bool { return false }
