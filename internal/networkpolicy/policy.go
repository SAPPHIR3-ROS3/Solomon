// Package networkpolicy stores Solomon's network selection independently of OS configuration.
package networkpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/gofrs/flock"
)

type Policy struct {
	Disabled         map[string]bool `json:"disabled"`
	DisabledMagicDNS map[string]bool `json:"disabled_magic_dns,omitempty"`
}
type Interface struct {
	Tailscale    *TailscaleInfo `json:"tailscale,omitempty"`
	Type         string         `json:"type"`
	DisplayLabel string         `json:"display_name"`
	Number       int            `json:"number"`
	Name         string         `json:"name"`
	Enabled      bool           `json:"enabled"`
	Up           bool           `json:"up"`
	Local        bool           `json:"local"`
	Addresses    []string       `json:"addresses"`
}

func PolicyPath() (string, error) { return policyPath() }

func ListUnfiltered(p Policy) ([]Interface, error) { return list(p) }

func WritePolicyFile(path string, p Policy) error { return writePolicy(path, p) }

func policyPath() (string, error) {
	home, err := paths.SolomonHome()
	return filepath.Join(home, "network.json"), err
}
func Read() (Policy, error) {
	path, err := policyPath()
	if err != nil {
		return Policy{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Policy{}, nil
	}
	if err != nil {
		return Policy{}, err
	}
	var p Policy
	err = json.Unmarshal(data, &p)
	return p, err
}
func List() ([]Interface, error) {
	p, err := Read()
	if err != nil {
		return nil, err
	}
	rows, err := list(p)
	if err != nil {
		return nil, err
	}
	rows = visibleInterfaces(rows, runningInContainer(os.ReadFile))
	for index := range rows {
		rows[index].Type = interfaceType(rows[index].Name, rows[index].Local)
		rows[index].DisplayLabel = rows[index].DisplayName()
	}
	enrichTailscale(rows)
	for index := range rows {
		row := &rows[index]
		if row.Tailscale != nil {
			row.Tailscale.MagicDNSEnabled = row.Enabled && row.Tailscale.MagicDNS != "" && p.AllowedHostname(row.Tailscale.MagicDNS)
		}
	}
	return rows, nil
}
func list(p Policy) ([]Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	sort.Slice(interfaces, func(i, j int) bool { return interfaces[i].Index < interfaces[j].Index })
	result := make([]Interface, 0, len(interfaces))
	for _, item := range interfaces {
		addresses, err := item.Addrs()
		if err != nil {
			return nil, err
		}
		row := Interface{Number: item.Index, Name: item.Name, Enabled: !p.Disabled[item.Name], Up: item.Flags&net.FlagUp != 0, Local: item.Flags&net.FlagLoopback != 0, Addresses: []string{}}
		if row.Local {
			row.Enabled = true
		}
		for _, address := range addresses {
			row.Addresses = append(row.Addresses, address.String())
		}
		result = append(result, row)
	}
	return result, nil
}
func Set(name string, enabled bool) error {
	path, err := policyPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	if err = lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	p, err := Read()
	if err != nil {
		return err
	}
	rows, err := list(p)
	if err != nil {
		return err
	}
	if parent, magicDNS := strings.CutSuffix(name, ":magicdns"); magicDNS {
		enrichTailscale(rows)
		for _, row := range rows {
			if row.Name != parent || row.Tailscale == nil || row.Tailscale.MagicDNS == "" {
				continue
			}
			if enabled && !row.Enabled {
				return fmt.Errorf("enable the parent Tailscale interface first")
			}
			if p.DisabledMagicDNS == nil {
				p.DisabledMagicDNS = map[string]bool{}
			}
			host := strings.ToLower(row.Tailscale.MagicDNS)
			for _, key := range []string{host, strings.SplitN(host, ".", 2)[0]} {
				if enabled {
					delete(p.DisabledMagicDNS, key)
				} else {
					p.DisabledMagicDNS[key] = true
				}
			}
			return writePolicy(path, p)
		}
		return fmt.Errorf("MagicDNS is not available for %q", parent)
	}
	found := false
	for _, row := range rows {
		if row.Name == name {
			found = true
			if row.Local && !enabled {
				return fmt.Errorf("local access must remain enabled")
			}
		}
	}
	if !found {
		return fmt.Errorf("unknown interface %q", name)
	}
	if p.Disabled == nil {
		p.Disabled = map[string]bool{}
	}
	if enabled {
		delete(p.Disabled, name)
	} else {
		p.Disabled[name] = true
	}
	return writePolicy(path, p)
}

func writePolicy(path string, p Policy) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".network-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// AllowedLocal checks the destination address accepted by the server.
func AllowedLocal(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}
	p, err := Read()
	if err != nil {
		return false
	}
	rows, err := list(p)
	if err != nil {
		return false
	}
	for _, row := range rows {
		for _, address := range row.Addresses {
			value, _, err := net.ParseCIDR(address)
			if err == nil && value.Equal(ip) {
				return row.Enabled
			}
		}
	}
	return false
}

// AllowedHostname controls Solomon access via the saved MagicDNS aliases.
func (p Policy) AllowedHostname(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return !p.DisabledMagicDNS[host]
}
