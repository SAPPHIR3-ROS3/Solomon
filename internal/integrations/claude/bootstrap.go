package claude

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed bundle
var bundle embed.FS

func install(ctx context.Context) (string, error) {
	dir, err := os.MkdirTemp("", "solomon-claude-")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	err = fs.WalkDir(bundle, "bundle", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(dir, strings.TrimPrefix(path, "bundle/"))
		if path == "bundle" {
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		data, err := bundle.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		return "", fmt.Errorf("extract Claude runtime: %w", err)
	}
	cmd := exec.CommandContext(ctx, "npm", "ci", "--omit=dev", "--no-audit", "--no-fund", "--ignore-scripts")
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcess(cmd) }
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("install locked Claude SDK: %w", err)
	}
	ok = true
	return dir, nil
}

func installedCLI() string {
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	names := []string{filepath.Join(home, ".local", "bin", "claude"), filepath.Join(home, ".claude", "local", "claude"), filepath.Join(home, ".npm-global", "bin", "claude")}
	if runtime.GOOS == "windows" {
		names = append(names, filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"), filepath.Join(home, ".local", "bin", "claude.exe"))
	}
	for _, path := range names {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
			return path
		}
	}
	return ""
}

func bundledCLI(dir string) (string, error) {
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	base := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk-"+platform+"-"+arch)
	if runtime.GOOS == "linux" {
		if matches, _ := filepath.Glob("/lib/ld-musl-*.so.1"); len(matches) > 0 {
			base += "-musl"
		}
	}
	path := filepath.Join(base, name)
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", fmt.Errorf("official Claude CLI missing from SDK optional package for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return path, nil
}

func Login(ctx context.Context, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cli := installedCLI()
	if cli == "" {
		dir, err := install(ctx)
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		cli, err = bundledCLI(dir)
		if err != nil {
			return err
		}
	}
	if out == nil {
		out = io.Discard
	}
	cmd := exec.CommandContext(ctx, cli, "auth", "login")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, out
	configureProcess(cmd)
	cmd.Cancel = func() error { return killProcess(cmd) }
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("official Claude CLI login: %w", err)
	}
	return nil
}
