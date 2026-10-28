package networkpolicy

// DisplayName keeps the actual interface name when its type cannot be established.
func (i Interface) DisplayName() string {
	switch i.Type {
	case "tailscale":
		return "Tailscale"
	case "ethernet":
		return "Ethernet"
	case "wifi":
		return "Wi-Fi"
	case "loopback":
		return "Localhost"
	case "bridge":
		return "Bridge"
	case "bond":
		return "Bond"
	case "wireguard":
		return "WireGuard"
	case "tunnel":
		return "Tunnel"
	case "vlan":
		return "VLAN"
	case "veth":
		return "Virtual Ethernet"
	default:
		return i.Name
	}
}
