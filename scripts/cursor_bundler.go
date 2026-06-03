//go:build ignore

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

type lineIO struct{}

func (lineIO) Print(msg string) { fmt.Println(msg) }

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "stop":
		err = cmdStop()
	case "build":
		err = cmdBuild(buildForce(os.Args))
	case "bundle":
		err = cmdBundle()
	case "install":
		err = cmdInstall()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run scripts/cursor_bundler.go <stop|build|build --force|bundle|install>")
}

func buildForce(args []string) bool {
	return len(args) > 2 && args[2] == "--force"
}

func cmdStop() error {
	if runtime.GOOS == "windows" {
		ps := `$p = Get-CimInstance Win32_Process -Filter "Name='node.exe'" -ErrorAction SilentlyContinue
foreach ($x in $p) {
  $c = $x.CommandLine
  if (-not $c) { continue }
  if ($c -like '*\integrations\cursor\*' -or $c -like '*\.solomon\integrations\cursor\*') {
    Write-Host ('Stopping node PID ' + $x.ProcessId)
    Stop-Process -Id $x.ProcessId -Force -ErrorAction SilentlyContinue
  }
}
Start-Sleep -Milliseconds 400`
		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	sh := `pkill -f '[/\\]integrations[/\\]cursor[/\\]' 2>/dev/null || true; pkill -f '[/\\]\.solomon[/\\]integrations[/\\]cursor[/\\]' 2>/dev/null || true`
	cmd := exec.Command("sh", "-c", sh)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdBuild(force bool) error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "integrations", "cursor")
	if !force && !cursorDistStale(dir) {
		fmt.Println("cursor integration dist up to date")
		return nil
	}
	if !cursorBuildDepsReady(dir) {
		if err := runNPM(dir, "install"); err != nil {
			return fmt.Errorf("%w (run: go run scripts/cursor_bundler.go stop, or remove integrations/cursor/node_modules)", err)
		}
	}
	return runNPM(dir, "run", "build")
}

func cmdBundle() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	src := filepath.Join(root, "integrations", "cursor")
	bundle := filepath.Join(root, "internal", "integrations", "cursor", "bundle")
	if err := os.RemoveAll(bundle); err != nil {
		return err
	}
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		if err := copyFile(filepath.Join(src, name), filepath.Join(bundle, name)); err != nil {
			return err
		}
	}
	npmrcDst := filepath.Join(bundle, ".npmrc")
	if err := copyFileOptional(filepath.Join(src, ".npmrc"), npmrcDst); err != nil {
		return err
	}
	if _, err := os.Stat(npmrcDst); os.IsNotExist(err) {
		if err := os.WriteFile(npmrcDst, []byte("loglevel=error\n"), 0o644); err != nil {
			return err
		}
	}
	distSrc := filepath.Join(src, "dist")
	distDst := filepath.Join(bundle, "dist")
	if err := copyDir(distSrc, distDst); err != nil {
		return fmt.Errorf("copy dist: %w", err)
	}
	fmt.Println("cursor bundle prepared at", bundle)
	return nil
}

func cmdInstall() error {
	if err := cmdStop(); err != nil {
		return err
	}
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	bundle := filepath.Join(root, "internal", "integrations", "cursor", "bundle")
	if _, err := os.Stat(filepath.Join(bundle, "dist", "index.js")); err != nil {
		return fmt.Errorf("bundle missing (run: go run scripts/cursor_bundler.go build && go run scripts/cursor_bundler.go bundle): %w", err)
	}
	dir, err := cursorInstallDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := copyDir(bundle, dir); err != nil {
		return err
	}
	fmt.Println("installing Cursor integration with Cursor SDK")
	if err := npmInstallProd(dir); err != nil {
		return err
	}
	fmt.Println("cursor integration installed at", dir)
	return nil
}

func cursorInstallDir() (string, error) {
	if p := strings.TrimSpace(os.Getenv("SOLOMON_CURSOR_API_ROOT")); p != "" {
		return p, nil
	}
	root, err := paths.SolomonHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "integrations", "cursor"), nil
}

func npmInstallProd(dir string) error {
	sdk := filepath.Join(dir, "node_modules", "@cursor", "sdk")
	if _, err := os.Stat(sdk); err == nil {
		return nil
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH: %w", err)
	}
	cmd := exec.Command(npm, "install", "--omit=dev")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install failed in %s: %w", dir, err)
	}
	return nil
}

func cursorDistStale(dir string) bool {
	dist := filepath.Join(dir, "dist", "index.js")
	distInfo, err := os.Stat(dist)
	if err != nil {
		return true
	}
	var newest time.Time
	for _, root := range []string{filepath.Join(dir, "src"), filepath.Join(dir, "prompts")} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			return nil
		})
	}
	if newest.IsZero() {
		return true
	}
	return newest.After(distInfo.ModTime())
}

func cursorBuildDepsReady(dir string) bool {
	for _, name := range []string{"esbuild", "esbuild.cmd"} {
		if _, err := os.Stat(filepath.Join(dir, "node_modules", ".bin", name)); err == nil {
			return true
		}
	}
	return false
}

func runNPM(dir string, args ...string) error {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH: %w", err)
	}
	cmd := exec.Command(npm, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	return nil
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyFileOptional(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
