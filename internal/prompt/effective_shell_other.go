//go:build !windows

package prompt

func windowsInteractiveShellOverride() string {
	return ""
}
