package commands

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/networkpolicy"
)

func Interface(d Deps, parts []string) error {
	rows, err := networkpolicy.List()
	if err != nil {
		return err
	}
	if len(parts) != 1 {
		if len(parts) != 3 || (parts[2] != "on" && parts[2] != "off") {
			return fmt.Errorf("usage: /interface | /interface <n> <on|off>")
		}
		magicDNS := strings.HasSuffix(parts[1], ".1")
		numberText := parts[1]
		if magicDNS {
			numberText = strings.TrimSuffix(numberText, ".1")
		}
		number, err := strconv.Atoi(numberText)
		if err != nil {
			return fmt.Errorf("invalid interface number")
		}
		found := false
		for _, row := range rows {
			if row.Number == number {
				found = true
				target := row.Name
				if magicDNS {
					target += ":magicdns"
				}
				if err := networkpolicy.Set(target, parts[2] == "on"); err != nil {
					return err
				}
			}
		}
		if !found {
			return fmt.Errorf("unknown interface number %d; run /interface", number)
		}
		rows, err = networkpolicy.List()
		if err != nil {
			return err
		}
	}
	out := d.Out
	if out == nil {
		out = os.Stdout
	}
	_, err = fmt.Fprint(out, formatInterfaceList(rows))
	return err
}

func FormatInterfaceList(rows []networkpolicy.Interface) string { return formatInterfaceList(rows) }

func formatInterfaceList(rows []networkpolicy.Interface) string {
	var output strings.Builder
	output.WriteString("Network interfaces\n\n")
	for _, row := range rows {
		state := "off"
		if row.Enabled {
			state = "on"
		}
		link := "down"
		if row.Up {
			link = "up"
		}
		label := row.DisplayName()
		if label != row.Name {
			label += " (" + row.Name + ")"
		}
		addresses := make([]string, 0, len(row.Addresses))
		for _, address := range row.Addresses {
			ip, _, err := net.ParseCIDR(address)
			if err == nil && ip.To4() != nil {
				addresses = append(addresses, address)
			}
		}
		fmt.Fprintf(&output, "%d. %s: %s (link %s) %s\n", row.Number, label, state, link, strings.Join(addresses, ", "))
		if row.Tailscale != nil {
			account := row.Tailscale.Account
			if row.Tailscale.Tagged {
				account = "tagged device (no user account)"
			}
			if account == "" {
				account = "unavailable"
			}
			fmt.Fprintf(&output, "   Account: %s | Tailnet: %s\n", account, row.Tailscale.Tailnet)
			if row.Tailscale.MagicDNS != "" {
				dnsState := "off"
				if row.Tailscale.MagicDNSEnabled {
					dnsState = "on"
				}
				fmt.Fprintf(&output, "   %d.1 MagicDNS: %s %s\n", row.Number, dnsState, row.Tailscale.MagicDNS)
			}
		}
	}
	output.WriteString("\n/interface <n> <on|off> · MagicDNS: <n>.1 · Local access is always enabled\n")
	return output.String()
}
