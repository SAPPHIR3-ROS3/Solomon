package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestNeedsStoppedPreparation(t *testing.T) {
	gui := filepath.Join(t.TempDir(), "gui")
	for _, tc := range []struct {
		name, goos, mode, dir string
		pid                   int
		ready, want           bool
	}{
		{"locked GUI", "windows", "dev", gui, 42, false, true},
		{"healthy GUI", "windows", "dev", gui, 42, true, false},
		{"other project", "windows", "dev", gui + "-other", 42, false, false},
		{"production", "windows", "normal", gui, 42, false, false},
		{"no daemon", "windows", "dev", gui, 0, false, false},
		{"unix", "linux", "dev", gui, 42, false, false},
		{"Windows path casing", "windows", "dev", strings.ToUpper(gui), 42, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := serverruntime.State{PID: tc.pid, Mode: tc.mode, DevDir: tc.dir}
			if got := needsStoppedPreparation(tc.goos, gui, state, func() bool { return tc.ready }); got != tc.want {
				t.Fatalf("needsStoppedPreparation = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoordinateInstallPreparationAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stop, fail bool
		want       []string
	}{
		{"normal", false, false, []string{"prepare", "stop", "deploy", "start"}},
		{"locked dependencies", true, false, []string{"stop", "prepare", "deploy", "start"}},
		{"normal preparation failure", false, true, []string{"prepare"}},
		{"locked preparation failure", true, true, []string{"stop", "prepare", "restore"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var steps []string
			failure := errors.New("npm ci failed")
			err := coordinateInstall(tc.stop, func() error {
				steps = append(steps, "prepare")
				if tc.fail {
					return failure
				}
				return nil
			}, func() error {
				steps = append(steps, "deploy")
				return nil
			}, func(install func() error) error {
				steps = append(steps, "stop")
				if err := install(); err != nil {
					steps = append(steps, "restore")
					return err
				}
				steps = append(steps, "start")
				return nil
			})
			if !reflect.DeepEqual(steps, tc.want) {
				t.Fatalf("steps = %v, want %v", steps, tc.want)
			}
			if errors.Is(err, failure) != tc.fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.fail && strings.Contains(err.Error(), "running service preserved") == tc.stop {
				t.Fatalf("incorrect preservation message: %v", err)
			}
		})
	}
}
