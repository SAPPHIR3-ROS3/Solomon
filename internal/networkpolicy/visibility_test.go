package networkpolicy

import (
	"os"
	"reflect"
	"testing"
)

func TestVisibleInterfacesOutsideAndInsideContainer(t *testing.T) {
	rows := []Interface{{Number: 1, Name: "lo"}, {Number: 2, Name: "eth0"}, {Number: 3, Name: "wlan0"}, {Number: 4, Name: "tailscale0"}, {Number: 5, Name: "wg0"}, {Number: 6, Name: "docker0"}, {Number: 7, Name: "vethabc123"}, {Number: 8, Name: "br-012345abcdef"}, {Number: 9, Name: "br-lan"}}
	want := append(append([]Interface{}, rows[:5]...), rows[8])
	if got := visibleInterfaces(rows, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("host list: %v", got)
	}
	if got := visibleInterfaces(rows, true); !reflect.DeepEqual(got, rows) {
		t.Fatalf("container list: %v", got)
	}
}

func TestRunningInContainer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"host", map[string]string{"/proc/self/cgroup": "0::/user.slice/user-1000.slice/session-2.scope"}, false},
		{"docker marker", map[string]string{"/.dockerenv": ""}, true},
		{"podman marker", map[string]string{"/run/.containerenv": ""}, true},
		{"docker cgroup", map[string]string{"/proc/self/cgroup": "1:cpu:/docker/012345"}, true},
		{"systemd docker", map[string]string{"/proc/1/cgroup": "0::/system.slice/docker-012345.scope"}, true},
		{"cgroup v2 without container marker", map[string]string{"/proc/self/cgroup": "0::/"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func(path string) ([]byte, error) {
				if data, ok := tc.files[path]; ok {
					return []byte(data), nil
				}
				return nil, os.ErrNotExist
			}
			if got := runningInContainer(read); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
