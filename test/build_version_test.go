package test

import (
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
	"testing"
)

func TestDevelopmentBaselineDoesNotRegress(t *testing.T) {
	for _, tc := range []struct {
		name, exact string
		dirty       bool
		candidates  []string
		want        string
	}{
		{"newer installed release", "", true, []string{"v2026.1009.0", "v2026.1011.2"}, "v2026.1011.2-dev"},
		{"repeated hot install", "", true, []string{"v2026.1009.0", "v2026.1011.2-dev-f1ba29a-dirty"}, "v2026.1011.2-dev"},
		{"newer published release", "", true, []string{"v2026.1009.0", "v2026.1011.2", "v2026.1012.0"}, "v2026.1012.0-dev"},
		{"calendar numeric ordering", "", false, []string{"v2026.930.0", "v2026.1009.0"}, "v2026.1009.0-dev"},
		{"exact clean release", "v2026.1011.2", false, []string{"v2026.1009.0"}, "v2026.1011.2"},
		{"modified exact release", "v2026.1011.2", true, nil, "v2026.1011.2-dev"},
		{"older exact checkout", "v2026.1009.0", false, []string{"v2026.1011.2"}, "v2026.1011.2-dev"},
		{"offline local fallback", "", true, []string{"v2026.1009.0"}, "v2026.1009.0-dev"},
		{"no release information", "", true, []string{"dev", "===SYSTEM===", "invalid"}, "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := updater.DevelopmentBuildVersion(tc.exact, tc.dirty, tc.candidates...); got != tc.want {
				t.Fatalf("version = %q, want %q", got, tc.want)
			}
		})
	}
}
