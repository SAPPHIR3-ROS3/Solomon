//go:build !windows

package shellutils

func windowsEffective() string {
	return ""
}
