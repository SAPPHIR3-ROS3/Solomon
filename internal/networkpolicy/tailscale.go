package networkpolicy

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
)

type TailscaleInfo struct {
	devices         []LinkDevice
	MagicDNSEnabled bool   `json:"magic_dns_enabled"`
	MagicDNS        string `json:"magic_dns,omitempty"`
	Tailnet         string `json:"tailnet,omitempty"`
	DNSName         string `json:"dns_name,omitempty"`
	Account         string `json:"account,omitempty"`
	Tagged          bool   `json:"tagged,omitempty"`
}

// This runs only for the display list, never while routing HTTP traffic.
func enrichTailscale(rows []Interface) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	started := time.Now()
	data, err := cmd.Output()
	logging.ProcessFinished(ctx, cmd, "network.tailscale", started, err)
	if err != nil {
		return
	}
	applyTailscaleStatus(rows, data)
}

func ApplyTailscaleStatus(rows []Interface, data []byte) { applyTailscaleStatus(rows, data) }

func applyTailscaleStatus(rows []Interface, data []byte) {
	var status struct {
		BackendState   string
		CurrentTailnet *struct {
			Name            string
			MagicDNSSuffix  string
			MagicDNSEnabled bool
		}
		Self *struct {
			DNSName      string
			UserID       int64
			TailscaleIPs []string
			Tags         []string
		}
		User map[string]struct{ LoginName string }
		Peer map[string]struct {
			Online       bool
			HostName     string
			DNSName      string
			TailscaleIPs []string
		}
	}
	if json.Unmarshal(data, &status) != nil || status.Self == nil || status.CurrentTailnet == nil {
		return
	}
	if status.BackendState != "Running" && status.BackendState != "Stopped" {
		return
	}
	for index := range rows {
		row := &rows[index]
		if row.Local {
			continue
		}
		matched := false
		for _, address := range row.Addresses {
			ip, _, err := net.ParseCIDR(address)
			if err != nil {
				continue
			}
			for _, ownIP := range status.Self.TailscaleIPs {
				if ip.Equal(net.ParseIP(ownIP)) {
					matched = true
				}
			}
		}
		if !matched {
			continue
		}
		info := &TailscaleInfo{Tailnet: status.CurrentTailnet.Name, DNSName: status.CurrentTailnet.MagicDNSSuffix, Tagged: len(status.Self.Tags) > 0}
		if status.BackendState == "Running" {
			for _, peer := range status.Peer {
				if !peer.Online || strings.EqualFold(peer.HostName, "funnel-ingress-node") {
					continue
				}
				hostname := strings.TrimSuffix(peer.DNSName, ".")
				if hostname == "" {
					hostname = peer.HostName
				}
				var addresses []netip.Addr
				for _, address := range peer.TailscaleIPs {
					if ip, err := netip.ParseAddr(address); err == nil {
						addresses = append(addresses, ip)
					}
				}
				if len(addresses) == 0 {
					continue
				}
				slices.SortFunc(addresses, netip.Addr.Compare)
				addresses = slices.Compact(addresses)
				device := LinkDevice{IP: addresses[0].String(), Hostname: hostname}
				for _, ip := range addresses {
					device.IPs = append(device.IPs, ip.String())
				}
				info.devices = append(info.devices, device)
			}
		}
		if status.BackendState == "Running" && status.CurrentTailnet.MagicDNSEnabled {
			name := strings.TrimSuffix(status.Self.DNSName, ".")
			if validMagicDNSName(name) {
				info.MagicDNS = name
			}
		}
		if !info.Tagged && status.Self.UserID != 0 {
			info.Account = status.User[strconv.FormatInt(status.Self.UserID, 10)].LoginName
		}
		row.Type = "tailscale"
		row.DisplayLabel = "Tailscale"
		row.Tailscale = info
	}
}

func validMagicDNSName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
				return false
			}
		}
	}
	return true
}
