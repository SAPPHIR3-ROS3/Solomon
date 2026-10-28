//go:build !linux

package networkpolicy

func interfaceType(name string, local bool) string {
	if local {
		return "loopback"
	}
	return "unknown"
}
