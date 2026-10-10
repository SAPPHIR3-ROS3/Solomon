package server

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func appendTailscaleMagicDNS(addresses []ReachableAddress, port int) []ReachableAddress {
	for _, address := range addresses {
		if address.Kind != "tailscale" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
		configureManagedProcess(cmd)
		data, err := cmd.Output()
		if err == nil {
			if dns, ok := tailscaleMagicDNSAddress(addresses, port, data); ok {
				return append(addresses, dns)
			}
		}
		break
	}
	return addresses
}

func tailscaleMagicDNSAddress(addresses []ReachableAddress, port int, data []byte) (ReachableAddress, bool) {
	var status struct {
		BackendState   string
		CurrentTailnet *struct{ MagicDNSEnabled bool }
		Self           *struct {
			DNSName      string
			TailscaleIPs []string
		}
	}
	if json.Unmarshal(data, &status) != nil || status.BackendState != "Running" || status.Self == nil || status.CurrentTailnet == nil || !status.CurrentTailnet.MagicDNSEnabled {
		return ReachableAddress{}, false
	}
	name := strings.TrimSuffix(status.Self.DNSName, ".")
	if name == "" || strings.ContainsAny(name, " /\\:@?#\r\n\t") {
		return ReachableAddress{}, false
	}
	for _, address := range addresses {
		if address.Kind != "tailscale" {
			continue
		}
		parsed, err := url.Parse(address.URL)
		if err != nil {
			continue
		}
		ip := net.ParseIP(parsed.Hostname())
		for _, selfIP := range status.Self.TailscaleIPs {
			if ip != nil && ip.Equal(net.ParseIP(selfIP)) {
				return ReachableAddress{
					Kind:      "tailscale",
					Interface: address.Interface,
					URL:       "http://" + net.JoinHostPort(name, strconv.Itoa(port)),
				}, true
			}
		}
	}
	return ReachableAddress{}, false
}
