package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/updater"
)

func requireDesktopWebView() error {
	if desktopWebViewAvailable() {
		return nil
	}
	switch runtime.GOOS {
	case "linux":
		return fmt.Errorf("solomon desktop cannot open its window because WebKitGTK 4.1 is not installed.\nInstall it, then run solomon desktop again:\n  Debian, Ubuntu: sudo apt-get install libgtk-3-0 libwebkit2gtk-4.1-0\n  Fedora: sudo dnf install gtk3 webkit2gtk4.1\nUntil then, run solomon for the terminal, or solomon server start and open the printed http://127.0.0.1 URL in a browser")
	case "windows":
		return fmt.Errorf("solomon desktop cannot open its window because the Microsoft Edge WebView2 Runtime is not installed.\nInstall the Evergreen Runtime from https://developer.microsoft.com/microsoft-edge/webview2/ and run solomon desktop again.\nUntil then, run solomon for the terminal, or solomon server start and open the printed http://127.0.0.1 URL in a browser")
	case "darwin":
		return fmt.Errorf("solomon desktop cannot open its window because /System/Library/Frameworks/WebKit.framework is missing.\nRestore WebKit with macOS software update, then run solomon desktop again.\nUntil then, run solomon for the terminal, or solomon server start and open the printed http://127.0.0.1 URL in a browser")
	default:
		return fmt.Errorf("solomon desktop cannot open its window because this system has no WebKit or WebView2 runtime.\nRun solomon for the terminal, or solomon server start and open the printed http://127.0.0.1 URL in a browser")
	}
}

func desktopWebViewAvailable() bool {
	switch runtime.GOOS {
	case "darwin":
		info, err := os.Stat("/System/Library/Frameworks/WebKit.framework")
		return err == nil && info.IsDir()
	case "linux":
		if _, err := exec.LookPath("pkg-config"); err == nil {
			if exec.Command("pkg-config", "--exists", "webkit2gtk-4.1").Run() == nil {
				return true
			}
		}
		for _, path := range []string{
			"/usr/lib/x86_64-linux-gnu/libwebkit2gtk-4.1.so.0",
			"/usr/lib/aarch64-linux-gnu/libwebkit2gtk-4.1.so.0",
			"/usr/lib64/libwebkit2gtk-4.1.so.0",
			"/usr/lib/libwebkit2gtk-4.1.so.0",
		} {
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				return true
			}
		}
		out, err := exec.Command("ldconfig", "-p").Output()
		return err == nil && strings.Contains(string(out), "libwebkit2gtk-4.1.so")
	case "windows":
		roots := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")}
		for _, root := range roots {
			if root == "" {
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(root, "Microsoft", "EdgeWebView", "Application", "*", "msedgewebview2.exe"))
			if len(matches) > 0 {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func desktopSetupRequired(args []string) bool {
	if len(args) < 2 {
		return true
	}
	switch args[1] {
	case "version":
		return len(args) < 3 || args[2] != "--binary"
	case "server", "sandbox-worker", "desktop":
		return false
	default:
		return true
	}
}

func launchDesktop(args []string) error {
	if len(args) > 0 && args[0] == "install" {
		return installDesktopCLI(args[1:])
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: solomon desktop [install [release-tag]]")
	}
	if err := requireDesktopWebView(); err != nil {
		return err
	}
	return launchDesktopWindow()
}

func installDesktopCLI(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: solomon desktop install [release-tag]")
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	ctx := context.Background()
	tag := "latest"
	if len(args) == 1 {
		tag = args[0]
	}
	if tag == "latest" {
		result := updater.Check(ctx, "dev")
		if result.Err != nil {
			return result.Err
		}
		tag = result.LatestTag
	}
	cliPath, err := os.Executable()
	if err != nil {
		return err
	}
	if err := updater.InstallDesktop(ctx, tag, cliPath, os.Stdout); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Solomon Desktop installed. Open it from the application menu or run: solomon desktop")
	return nil
}
