package webfetch

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sync"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 (+https://github.com/SAPPHIR3-ROS3/Solomon)"
)

var (
	clientMu     sync.RWMutex
	sharedClient *http.Client
	maxRedirects = config.DefaultWebFetchMaxRedirects
)

func sharedHTTPClient() *http.Client {
	clientMu.RLock()
	c := sharedClient
	clientMu.RUnlock()
	if c != nil {
		return c
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if sharedClient == nil {
		sharedClient = newHTTPClient(maxRedirects)
	}
	return sharedClient
}

func newHTTPClient(redirectLimit int) *http.Client {
	if redirectLimit <= 0 {
		redirectLimit = config.DefaultWebFetchMaxRedirects
	}
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= redirectLimit {
				return fmt.Errorf("stopped after %d redirects", redirectLimit)
			}
			return nil
		},
	}
}

func configureRedirects(n int) {
	if n <= 0 {
		n = config.DefaultWebFetchMaxRedirects
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	if n == maxRedirects && sharedClient != nil {
		return
	}
	maxRedirects = n
	sharedClient = newHTTPClient(n)
}

func effectiveUserAgent(cfg *config.Root) string {
	if s := config.EffectiveWebFetchUserAgent(cfg); s != "" {
		return s
	}
	return defaultUserAgent
}
