package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"solomon/internal/agent"
	"solomon/internal/chatstore"
	"solomon/internal/config"
	"solomon/internal/logging"
	"solomon/internal/paths"
	"solomon/internal/project"
	"solomon/internal/termcolor"

	"github.com/chzyer/readline"
)

func main() {
	ctx := context.Background()
	lroot, err := paths.SolomonHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	initLogLevel := logging.INFO_LOG_LEVEL
	if cfg0, err := config.Load(); err == nil && cfg0.LogLevel != "" {
		if lvl, err := logging.ParseLevel(cfg0.LogLevel); err == nil {
			initLogLevel = lvl
		}
	}
	logging.LogInit(initLogLevel)
	if err := logging.Configure(logging.Config{
		Dir: filepath.Join(lroot, "logs"), WriteConsole: false, WriteFile: true, Retention: 7,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := config.RunWizardIfNeeded(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if cfg.Current.Model == "" {
		p, err := config.ResolveProvider(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		mid, err := config.PickModelInteractive(os.Stdin, p, p.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "model selection failed: %v\nSet current.model in ~/.solomon/config.toml\n", err)
			os.Exit(1)
		}
		cfg.Current.Model = mid
		cfg.Current.Provider = p.Name
		if err := config.Save(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	prov, err := config.ResolveProvider(cfg)
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
	rl, err := readline.NewEx(&readline.Config{
		Prompt: termcolor.User + "You: " + termcolor.Reset,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rl.Close()

	sess := &chatstore.Session{
		ID:             "",
		Title:          "",
		CreatedAt:      time.Now(),
		LastMessageAt:  time.Now(),
		Messages:       nil,
	}
	rt := agent.NewRuntime(rl, cfg, prov, hex, root, sess)
	if len(os.Args) >= 4 && os.Args[1] == "temp" && os.Args[2] == "exec" {
		rt.EphemeralSession = true
		prompt := strings.TrimSpace(strings.Join(os.Args[3:], " "))
		if prompt == "" {
			fmt.Fprintln(os.Stderr, `usage: solomon temp exec <prompt>`)
			os.Exit(1)
		}
		if err := rt.RunPromptOnce(ctx, prompt); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "exec" {
		prompt := strings.TrimSpace(strings.Join(os.Args[2:], " "))
		if prompt == "" {
			fmt.Fprintln(os.Stderr, `usage: solomon exec <prompt>  (shell quotes grouping text, not passed to Solomon)`)
			os.Exit(1)
		}
		if err := rt.RunPromptOnce(ctx, prompt); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := rt.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
