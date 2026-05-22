package config

import "time"

const DefaultAPIMaxRetries = 3

const DefaultAPIBaseDelayMS = 1000

const DefaultAPIMaxDelayMS = 30000

const DefaultAPIConnectTimeoutSec = 30

const DefaultAPICircuitOpenSec = 60

type APIResilienceConfig struct {
	MaxRetries          int  `toml:"max_retries,omitempty"`
	BaseDelayMS         int  `toml:"base_delay_ms,omitempty"`
	MaxDelayMS          int  `toml:"max_delay_ms,omitempty"`
	Jitter              *bool `toml:"jitter,omitempty"`
	ConnectTimeoutSec   int  `toml:"connect_timeout_sec,omitempty"`
	ReadTimeoutSec      int  `toml:"read_timeout_sec,omitempty"`
	CircuitOpenSec      int  `toml:"circuit_open_sec,omitempty"`
}

type APIResiliencePolicy struct {
	MaxRetries        int
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	Jitter            bool
	ConnectTimeout    time.Duration
	ReadTimeout       time.Duration
	CircuitOpen       time.Duration
}

func EffectiveAPIResilience(r *Root) APIResiliencePolicy {
	p := APIResiliencePolicy{
		MaxRetries:     DefaultAPIMaxRetries,
		BaseDelay:      time.Duration(DefaultAPIBaseDelayMS) * time.Millisecond,
		MaxDelay:       time.Duration(DefaultAPIMaxDelayMS) * time.Millisecond,
		Jitter:         true,
		ConnectTimeout: time.Duration(DefaultAPIConnectTimeoutSec) * time.Second,
		ReadTimeout:    0,
		CircuitOpen:    time.Duration(DefaultAPICircuitOpenSec) * time.Second,
	}
	if r == nil {
		return p
	}
	c := r.APIResilience
	if c.MaxRetries > 0 {
		if c.MaxRetries > 10 {
			p.MaxRetries = 10
		} else {
			p.MaxRetries = c.MaxRetries
		}
	}
	if c.BaseDelayMS > 0 {
		p.BaseDelay = time.Duration(c.BaseDelayMS) * time.Millisecond
	}
	if c.MaxDelayMS > 0 {
		p.MaxDelay = time.Duration(c.MaxDelayMS) * time.Millisecond
	}
	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}
	if c.Jitter != nil {
		p.Jitter = *c.Jitter
	}
	if c.ConnectTimeoutSec > 0 {
		if c.ConnectTimeoutSec > 300 {
			p.ConnectTimeout = 300 * time.Second
		} else {
			p.ConnectTimeout = time.Duration(c.ConnectTimeoutSec) * time.Second
		}
	}
	if c.ReadTimeoutSec > 0 {
		if c.ReadTimeoutSec > 3600 {
			p.ReadTimeout = 3600 * time.Second
		} else {
			p.ReadTimeout = time.Duration(c.ReadTimeoutSec) * time.Second
		}
	}
	if c.CircuitOpenSec > 0 {
		if c.CircuitOpenSec > 600 {
			p.CircuitOpen = 600 * time.Second
		} else {
			p.CircuitOpen = time.Duration(c.CircuitOpenSec) * time.Second
		}
	}
	return p
}
