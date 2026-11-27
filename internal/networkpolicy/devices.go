package networkpolicy

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

func SetDevice(name, address string, allowed bool) error {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() {
		return fmt.Errorf("invalid device IPv4 address")
	}
	if _, err := net.InterfaceByName(name); err != nil {
		return err
	}
	path, err := policyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	p, err := Read()
	if err != nil {
		return err
	}
	if !allowed {
		delete(p.AllowedDevices[name], ip.String())
		return writePolicy(path, p)
	}
	if p.AllowedDevices == nil {
		p.AllowedDevices = make(map[string]map[string]bool)
	}
	if p.AllowedDevices[name] == nil {
		p.AllowedDevices[name] = make(map[string]bool)
	}
	p.AllowedDevices[name][ip.String()] = true
	return writePolicy(path, p)
}

func (p Policy) AllowsDevice(destination, source net.IP) bool {
	ip := source.To4()
	if ip == nil || destination.IsLoopback() {
		return false
	}
	rows, err := list(p)
	if err != nil {
		return false
	}
	for _, row := range rows {
		if !row.Enabled || !p.AllowedDevices[row.Name][ip.String()] {
			continue
		}
		for _, address := range row.Addresses {
			local, _, err := net.ParseCIDR(address)
			if err == nil && destination.Equal(local) {
				return true
			}
		}
	}
	return false
}
