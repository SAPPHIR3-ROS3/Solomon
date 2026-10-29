package server

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"net"
	"sort"
	"strconv"
	"strings"
)

func resolveListenAddr(options Options) (string, error) {
	if options.ListenAddr != "" {
		return options.ListenAddr, nil
	}

	port, err := config.ResolveServerPort()
	if err != nil {
		return "", err
	}
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(port)), nil
}

func reachableAddresses(listenerAddr *net.TCPAddr, port int) []ReachableAddress {
	if listenerAddr == nil {
		return nil
	}

	addresses := make([]ReachableAddress, 0)
	seen := make(map[string]bool)
	appendAddress := func(ip net.IP, interfaceName string) {
		ip = ip.To4()
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			return
		}
		key := ip.String()
		if seen[key] {
			return
		}
		seen[key] = true
		kind := "local"
		if isTailscaleIPv4(ip) {
			kind = "tailscale"
		}
		addresses = append(addresses, ReachableAddress{
			Kind:      kind,
			Interface: interfaceName,
			URL:       "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port)),
		})
	}

	listenerIP := listenerAddr.IP.To4()
	if listenerIP == nil || listenerIP.IsLoopback() {
		return nil
	}
	if !listenerIP.IsUnspecified() {
		appendAddress(listenerIP, interfaceNameForIP(listenerIP))
		return appendTailscaleMagicDNS(addresses, port)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || isContainerInterface(networkInterface.Name) {
			continue
		}
		interfaceAddrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, interfaceAddr := range interfaceAddrs {
			appendAddress(interfaceIPv4(interfaceAddr), networkInterface.Name)
		}
	}

	addresses = appendTailscaleMagicDNS(addresses, port)
	sort.Slice(addresses, func(i, j int) bool {
		if addresses[i].Kind != addresses[j].Kind {
			return addresses[i].Kind == "local"
		}
		if addresses[i].Interface != addresses[j].Interface {
			return addresses[i].Interface < addresses[j].Interface
		}
		return addresses[i].URL < addresses[j].URL
	})
	return addresses
}

// Container bridges and virtual Ethernet pairs are not useful client URLs.
// Keep LAN interfaces and VPNs (including WireGuard and Tailscale) visible.
func isContainerInterface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth", "cni", "flannel", "podman"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func interfaceIPv4(address net.Addr) net.IP {
	switch value := address.(type) {
	case *net.IPNet:
		return value.IP.To4()
	case *net.IPAddr:
		return value.IP.To4()
	}

	text := address.String()
	if host, _, err := net.SplitHostPort(text); err == nil {
		return net.ParseIP(host).To4()
	}
	return net.ParseIP(text).To4()
}

func interfaceNameForIP(target net.IP) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, networkInterface := range interfaces {
		interfaceAddrs, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, interfaceAddr := range interfaceAddrs {
			if ip := interfaceIPv4(interfaceAddr); ip != nil && ip.Equal(target) {
				return networkInterface.Name
			}
		}
	}
	return ""
}

func isTailscaleIPv4(ip net.IP) bool {
	ip = ip.To4()
	return ip != nil && ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127
}
