package cloak

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func obscuraEndpoint(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	endpoint, err := url.Parse(value)
	if err == nil {
		ip := net.ParseIP(endpoint.Hostname())
		port, portErr := strconv.ParseUint(endpoint.Port(), 10, 16)
		if endpoint.Scheme == "ws" && ip != nil && ip.IsLoopback() &&
			portErr == nil && port > 0 && endpoint.User == nil &&
			endpoint.RawQuery == "" && !endpoint.ForceQuery && endpoint.Fragment == "" {
			return endpoint.String(), nil
		}
	}
	return "", errors.New("SOLOMON_OBSCURA_CDP_URL requires a loopback ws URL with an explicit port and no credentials")
}
