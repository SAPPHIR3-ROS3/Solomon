package test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/memoryprofile"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/processidentity"
)

func TestMemoryProfileDesktopHelper(t *testing.T) {
	childPath := os.Getenv("SOLOMON_MEMORY_TEST_CHILD")
	if childPath == "" {
		return
	}
	child := exec.Command(childPath, "60")
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	fmt.Println(child.Process.Pid)
	child.Wait()
	os.Exit(0)
}

func TestMemoryProfileIsolatesRegisteredDesktopTree(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, "run", "clients")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	copyBinary := func(source, name string) string {
		t.Helper()
		in, err := os.Open(source)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		name = filepath.Join(home, name)
		out, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(out, in)
		out.Close()
		if err != nil {
			t.Fatal(err)
		}
		return name
	}
	executable, _ := os.Executable()
	rootPath := copyBinary(executable, "solomon-desktop")
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep unavailable")
	}
	webkitPath := copyBinary(sleep, "WebKitWebProcess")
	unrelated := exec.Command(webkitPath, "60")
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { unrelated.Process.Kill(); unrelated.Wait() }()
	root := exec.Command(rootPath, "-test.run=^TestMemoryProfileDesktopHelper$")
	root.Env = append(os.Environ(), "SOLOMON_MEMORY_TEST_CHILD="+webkitPath)
	output, err := root.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { root.Process.Kill(); root.Wait() }()
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() {
		t.Fatal("desktop helper did not start")
	}
	childPID, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	child, _ := os.FindProcess(childPID)
	defer child.Kill()
	record := memoryprofile.Process{PID: root.Process.Pid, Identity: processidentity.Read(root.Process.Pid), Executable: rootPath, Kind: "desktop"}
	publish := func(record memoryprofile.Process) {
		data, _ := json.Marshal(record)
		if err := os.WriteFile(filepath.Join(directory, "desktop.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	collector := memoryprofile.New(home)
	collector.Sample()
	sample := collector.Report().Samples[0]
	if sample.Desktop.Processes != 0 || sample.WebKit.Processes != 0 {
		t.Fatal("included unregistered processes")
	}
	publish(record)
	collector.Sample()
	sample = collector.Report().Samples[1]
	if sample.Server.Processes != 1 || sample.Desktop.Processes != 1 || sample.WebKit.Processes != 1 {
		t.Fatalf("scope: %+v", sample)
	}
	record.Identity = "reused-pid"
	publish(record)
	collector.Sample()
	sample = collector.Report().Samples[2]
	if sample.Desktop.Processes != 0 || sample.WebKit.Processes != 0 {
		t.Fatal("accepted stale process identity")
	}
	record.Identity = processidentity.Read(root.Process.Pid)
	record.Executable = copyBinary(sleep, "solomon")
	publish(record)
	collector.Sample()
	sample = collector.Report().Samples[3]
	if sample.Desktop.Processes != 0 || sample.WebKit.Processes != 0 {
		t.Fatal("accepted mismatched executable")
	}
}
