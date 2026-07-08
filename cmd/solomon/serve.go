package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

type serveCLIOpts struct {
	Addr      string
	StaticDir string
	NoStatic  bool
}

func parseServeArgs(args []string, cfg *config.Root) (serveCLIOpts, error) {
	opts := serveCLIOpts{Addr: cfg.EffectiveServerAddr()}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bind":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--bind requires a value")
			}
			i++
			addr, err := config.ParseServeAddrOverride(args[i])
			if err != nil {
				return opts, err
			}
			if addr != "" {
				opts.Addr = addr
			}
		case "--static-dir":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--static-dir requires a value")
			}
			i++
			opts.StaticDir = args[i]
		case "--no-static":
			opts.NoStatic = true
		default:
			return opts, fmt.Errorf("unknown serve flag %q", args[i])
		}
	}
	return opts, nil
}

func runServeCLI(ctx context.Context, args []string, preloaded *config.Root) {
	if config.NeedsOnboard(preloaded) {
		fmt.Fprintln(os.Stderr, "config not set up; run solomon and use /onboard")
		os.Exit(1)
	}
	prov, err := config.ResolveProvider(preloaded)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	opts, err := parseServeArgs(args, preloaded)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root, hex, err := project.Resolve(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = root
	cert, key := preloaded.EffectiveServerTLSPaths()
	srv, err := server.New(preloaded, prov, hex, root, server.Options{
		Addr:      opts.Addr,
		StaticDir: opts.StaticDir,
		NoStatic:  opts.NoStatic,
		CertPath:  cert,
		KeyPath:   key,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bootstrap, err := srv.BootstrapToken()
	if err == nil && bootstrap != "" {
		fmt.Fprintf(os.Stderr, "Bootstrap token (one-time): %s\n", bootstrap)
		fmt.Fprintln(os.Stderr, "Exchange via POST /v1/auth/bootstrap then use Authorization: Bearer <token>")
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-ctx.Done():
	case <-sig:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
