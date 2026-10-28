package networkpolicy

import (
	"os"
	"path/filepath"
	"strings"
)

func interfaceType(name string, local bool) string {
	if local {
		return "loopback"
	}
	return linuxInterfaceType("/sys/class/net", name)
}

// Linux's sysfs type is ARPHRD, not IANA ifType. Wi-Fi, bridges and virtual
// Ethernet can all report ARPHRD_ETHER, so inspect device metadata first.
func linuxInterfaceType(root, name string) string {
	base := filepath.Join(root, name)
	exists := func(entry string) bool { _, err := os.Stat(filepath.Join(base, entry)); return err == nil }
	data, _ := os.ReadFile(filepath.Join(base, "uevent"))
	deviceType := ""
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "DEVTYPE="); ok {
			deviceType = value
		}
	}
	if deviceType == "wlan" || exists("phy80211") || exists("wireless") {
		return "wifi"
	}
	switch deviceType {
	case "bridge", "bond", "wireguard", "vlan", "veth":
		return deviceType
	}
	if exists("bridge") {
		return "bridge"
	}
	if exists("bonding") {
		return "bond"
	}
	if exists("tun_flags") {
		return "tunnel"
	}
	data, err := os.ReadFile(filepath.Join(base, "type"))
	if err != nil {
		return "unknown"
	}
	switch strings.TrimSpace(string(data)) {
	case "772":
		return "loopback" // ARPHRD_LOOPBACK
	case "1": // ARPHRD_ETHER alone does not establish wired hardware.
		if exists("device") {
			return "ethernet"
		}
	case "768", "769", "776", "778", "823":
		return "tunnel"
	}
	return "unknown"
}
