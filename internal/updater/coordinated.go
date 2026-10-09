package updater

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

// A detached coordinator must outlive the daemon-owned PTY requesting /upgrade.
func launchCoordinatedUpdate(tag string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	installedCLI := executable
	if configured := os.Getenv("SOLOMON_BINARY"); configured != "" {
		installedCLI = configured
	}
	home, err := paths.SolomonHome()
	if err != nil {
		return err
	}
	logDir := filepath.Join(home, "logs", "update")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(logDir, "update.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	// A running .exe locks its own file. Run the helper from a separate copy.
	if runtime.GOOS == "windows" {
		// Remove copies left by previous completed helpers; running .exe files
		// remain locked and cannot be removed here.
		oldCopies, _ := filepath.Glob(filepath.Join(logDir, "coordinator-*.exe"))
		for _, old := range oldCopies {
			_ = os.Remove(old)
		}
		source, err := os.Open(executable)
		if err != nil {
			return err
		}
		defer source.Close()
		copy, err := os.CreateTemp(logDir, "coordinator-*.exe")
		if err != nil {
			return err
		}
		_, err = io.Copy(copy, source)
		closeErr := copy.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(copy.Name())
			return err
		}
		executable = copy.Name()
	}
	args := []string{"__upgrade-runtime", tag, strconv.Itoa(os.Getpid())}
	command := exec.Command(executable, args...)
	command.Env = append(os.Environ(), "SOLOMON_BINARY="+installedCLI)
	command.Stdout, command.Stderr = log, log
	configureCoordinator(command)
	if err := command.Start(); err != nil {
		if runtime.GOOS == "windows" {
			_ = os.Remove(executable)
		}
		return err
	}
	_ = command.Process.Release()
	fmt.Fprintf(os.Stdout, "Solomon update coordinator started; progress: %s\n", log.Name())
	return nil
}
