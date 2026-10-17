package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
)

func TestProviderEndpoints(t *testing.T) {
	for _, tc := range []struct {
		input, openai, anthropic string
	}{
		{"192.168.1.10:1234", "http://192.168.1.10:1234/v1/", "http://192.168.1.10:1234/"},
		{" localhost:11434/v1 ", "http://localhost:11434/v1/", "http://localhost:11434/v1/"},
		{"127.0.0.1", "http://127.0.0.1/v1/", "http://127.0.0.1/"},
		{"[::1]:8080/api", "http://[::1]:8080/api/v1/", "http://[::1]:8080/api/"},
		{"inference.local:8000", "http://inference.local:8000/v1/", "http://inference.local:8000/"},
		{"https://192.168.1.10:8443/v1", "https://192.168.1.10:8443/v1/", "https://192.168.1.10:8443/v1/"},
		{"https://api.example.com", "https://api.example.com/v1/", "https://api.example.com/"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			for _, protocol := range []struct {
				name      string
				normalize func(string) (string, error)
				want      string
			}{
				{"openai", config.NormalizeAPIBase, tc.openai},
				{"anthropic", config.NormalizeAnthropicBase, tc.anthropic},
			} {
				got, err := protocol.normalize(tc.input)
				if err != nil || got != protocol.want {
					t.Errorf("%s: got %q, %v; want %q", protocol.name, got, err, protocol.want)
				}
			}
		})
	}
}

func TestProviderEndpointsRejectInvalidInput(t *testing.T) {
	for _, input := range []string{"", "ftp://127.0.0.1:8000", "127.0.0.1:abc", "localhost:0", "localhost:65536", "not a host:8000"} {
		for _, normalize := range []func(string) (string, error){config.NormalizeAPIBase, config.NormalizeAnthropicBase} {
			if got, err := normalize(input); err == nil {
				t.Errorf("accepted %q as %q", input, got)
			}
		}
	}
}
