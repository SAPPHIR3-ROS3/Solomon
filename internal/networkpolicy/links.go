package networkpolicy

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

type LinkDevice struct {
	Allowed  bool     `json:"allowed"`
	IP       string   `json:"ip"`
	IPs      []string `json:"ips,omitempty"`
	MAC      string   `json:"mac,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
}

type Link struct {
	Name    string       `json:"name"`
	Label   string       `json:"label"`
	Status  string       `json:"status"`
	Devices []LinkDevice `json:"devices"`
}

func ListLinks(ctx context.Context) ([]Link, error) {
	interfaces, err := List()
	if err != nil {
		return nil, err
	}
	neighbors, gateways, status := linkNeighbors(ctx)
	localIPs := make(map[netip.Addr]bool)
	for _, iface := range interfaces {
		for _, address := range iface.Addresses {
			if prefix, err := netip.ParsePrefix(address); err == nil {
				localIPs[prefix.Addr().Unmap()] = true
			}
		}
	}
	links := make([]Link, 0, len(interfaces))
	for _, iface := range interfaces {
		if !iface.Enabled || !iface.Up || len(iface.Addresses) == 0 {
			continue
		}
		link := Link{Name: iface.Name, Label: iface.DisplayLabel, Status: status, Devices: neighbors[iface.Name]}
		switch {
		case iface.Local:
			link.Status, link.Devices = "local", nil
		case iface.Tailscale != nil:
			link.Status, link.Devices = "available", iface.Tailscale.devices
		}
		link.Devices = slices.DeleteFunc(link.Devices, func(device LinkDevice) bool {
			ip := netip.MustParseAddr(device.IP).Unmap()
			return ip.IsLoopback() || localIPs[ip]
		})
		slices.SortFunc(link.Devices, func(a, b LinkDevice) int {
			return netip.MustParseAddr(a.IP).Compare(netip.MustParseAddr(b.IP))
		})
		if link.Devices == nil {
			link.Devices = []LinkDevice{}
		}
		links = append(links, link)
	}
	resolveNeighborHostnames(ctx, links, gateways)
	return links, nil
}

func resolveNeighborHostnames(ctx context.Context, links []Link, gateways map[string]map[netip.Addr]bool) {
	type neighbor struct {
		device  *LinkDevice
		routers []netip.Addr
	}
	var neighbors []neighbor
	for _, link := range links {
		var routers []netip.Addr
		for router := range gateways[link.Name] {
			routers = append(routers, router)
		}
		slices.SortFunc(routers, netip.Addr.Compare)
		for index := range link.Devices {
			device := &link.Devices[index]
			if device.MAC != "" && netip.MustParseAddr(device.IP).Is4() {
				neighbors = append(neighbors, neighbor{device, routers})
			}
		}
	}
	if len(neighbors) == 0 {
		return
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	avahi, _ := exec.LookPath("avahi-resolve-address")
	workers := min(8, len(neighbors))
	var group sync.WaitGroup
	var advertised map[[2]string]string
	group.Go(func() { advertised = advertisedHostnames(lookupCtx) })
	for worker := range workers {
		group.Go(func() {
			for index := worker; index < len(neighbors) && lookupCtx.Err() == nil; index += workers {
				neighbor := neighbors[index]
				neighbor.device.Hostname = lookupNeighborHostname(lookupCtx, avahi, neighbor.device.IP, neighbor.routers)
			}
		})
	}
	group.Wait()
	for _, link := range links {
		for index := range link.Devices {
			device := &link.Devices[index]
			if device.MAC != "" && device.Hostname == "" {
				device.Hostname = advertised[[2]string{link.Name, device.IP}]
			}
		}
	}
}

func advertisedHostnames(ctx context.Context) map[[2]string]string {
	path, err := exec.LookPath("avahi-browse")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, _ := exec.CommandContext(ctx, path, "-artp").Output()
	names := make(map[[2]string]string)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Split(line, ";")
		if len(fields) < 8 || fields[0] != "=" || fields[2] != "IPv4" {
			continue
		}
		if name := neighborHostname(fields[6]); name != "" {
			names[[2]string{fields[1], fields[7]}] = name
		}
	}
	return names
}

func lookupNeighborHostname(ctx context.Context, avahi, ip string, routers []netip.Addr) string {
	if name := reverseHostname(ctx, net.DefaultResolver, ip); name != "" {
		return name
	}
	if avahi != "" {
		mdnsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		output, err := exec.CommandContext(mdnsCtx, avahi, ip).Output()
		cancel()
		fields := strings.Fields(string(output))
		if err == nil && len(fields) == 2 && fields[0] == ip {
			if name := neighborHostname(fields[1]); name != "" {
				return name
			}
		}
	}
	for _, router := range routers {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, netip.AddrPortFrom(router, 53).String())
			},
		}
		if name := reverseHostname(ctx, resolver, ip); name != "" {
			return name
		}
	}
	return ""
}

func reverseHostname(ctx context.Context, resolver *net.Resolver, ip string) string {
	dnsCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	names, err := resolver.LookupAddr(dnsCtx, ip)
	cancel()
	if err == nil {
		for _, name := range names {
			if hostname := neighborHostname(name); hostname != "" {
				return hostname
			}
		}
	}
	return ""
}

func neighborHostname(name string) string {
	name = strings.TrimSuffix(name, ".")
	switch strings.ToLower(name) {
	case "localhost", "localhost.local", "localhost.localdomain":
		return ""
	default:
		return name
	}
}

func linkNeighbors(ctx context.Context) (map[string][]LinkDevice, map[string]map[netip.Addr]bool, string) {
	if runtime.GOOS != "linux" {
		return nil, nil, "unsupported"
	}
	path, err := exec.LookPath("ip")
	if err != nil {
		return nil, nil, "unsupported"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, path, "-j", "neigh", "show").Output()
	if err != nil {
		return nil, nil, "unavailable"
	}
	var entries []struct {
		Destination string   `json:"dst"`
		Interface   string   `json:"dev"`
		MAC         string   `json:"lladdr"`
		State       []string `json:"state"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, nil, "unavailable"
	}
	gateways := linkGateways(ctx, path)
	devices := make(map[string][]LinkDevice)
	for _, entry := range entries {
		ip, err := netip.ParseAddr(entry.Destination)
		mac, macErr := net.ParseMAC(entry.MAC)
		if err != nil || macErr != nil || !ip.IsGlobalUnicast() && !ip.IsLinkLocalUnicast() ||
			gateways[entry.Interface][ip] ||
			slices.Contains(entry.State, "FAILED") || slices.Contains(entry.State, "INCOMPLETE") {
			continue
		}
		devices[entry.Interface] = append(devices[entry.Interface], LinkDevice{IP: ip.String(), MAC: mac.String()})
	}
	return devices, gateways, "available"
}

func linkGateways(ctx context.Context, path string) map[string]map[netip.Addr]bool {
	data, err := exec.CommandContext(ctx, path, "-j", "-4", "route", "show", "table", "all", "default").Output()
	if err != nil {
		return nil
	}
	type nextHop struct {
		Gateway string `json:"gateway"`
		Device  string `json:"dev"`
	}
	var routes []struct {
		nextHop
		NextHops []nextHop `json:"nexthops"`
	}
	if json.Unmarshal(data, &routes) != nil {
		return nil
	}
	gateways := make(map[string]map[netip.Addr]bool)
	for _, route := range routes {
		for _, hop := range append(route.NextHops, route.nextHop) {
			ip, err := netip.ParseAddr(hop.Gateway)
			if err != nil || hop.Device == "" {
				continue
			}
			if gateways[hop.Device] == nil {
				gateways[hop.Device] = make(map[netip.Addr]bool)
			}
			gateways[hop.Device][ip] = true
		}
	}
	return gateways
}
