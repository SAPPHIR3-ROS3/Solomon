//go:build !linux

package networkpolicy

func InterfaceType(name string, local bool) string { return interfaceType(name, local) }

func interfaceType(name string, local bool) string {
	if local {
		return "loopback"
	}
	return "unknown"
}
