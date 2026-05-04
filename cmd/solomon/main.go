package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"

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
		logging.Log(logging.ERROR_LOG_LEVEL, "configure logging failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	logging.Log(logging.INFO_LOG_LEVEL, "Solomon starting")
	if len(os.Args) >= 2 && os.Args[1] == "add" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "get working directory failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		root, hex, err := project.Resolve(wd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "resolve project failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, `usage: solomon add npx ... | skills.sh | skill <.md> [name] [global|project|local]`)
			logging.Log(logging.WARNING_LOG_LEVEL, "CLI add: invalid usage")
			os.Exit(2)
		}
		deps := commands.Deps{
			Ctx:      ctx,
			Out:      os.Stdout,
			Stdin:    os.Stdin,
			ProjHex:  hex,
			ProjRoot: root,
		}
		logging.Log(logging.INFO_LOG_LEVEL, "CLI skill add")
		if err := commands.Add(deps, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "CLI skill add failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "remove" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "get working directory failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		root, hex, err := project.Resolve(wd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "resolve project failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, `usage: solomon remove skill <name>`)
			logging.Log(logging.WARNING_LOG_LEVEL, "CLI remove: invalid usage")
			os.Exit(2)
		}
		deps := commands.Deps{
			Ctx:      ctx,
			Out:      os.Stdout,
			Stdin:    os.Stdin,
			ProjHex:  hex,
			ProjRoot: root,
		}
		logging.Log(logging.INFO_LOG_LEVEL, "CLI skill remove")
		if err := commands.Remove(deps, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "CLI skill remove failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		return
	}
	cfg, err := config.RunWizardIfNeeded(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "config setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	if cfg.Current.Model == "" {
		p, err := config.ResolveProvider(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "resolve provider failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		mid, err := config.PickModelInteractive(os.Stdin, p, p.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "model selection failed: %v\nSet current.model in ~/.solomon/config.toml\n", err)
			logging.Log(logging.ERROR_LOG_LEVEL, "model selection failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		cfg.Current.Model = mid
		cfg.Current.Provider = p.Name
		if err := config.Save(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "save config failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
	}
	prov, err := config.ResolveProvider(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "resolve provider failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "get working directory failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	root, hex, err := project.Resolve(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "resolve project failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	_ = root
	logging.Log(logging.INFO_LOG_LEVEL, "interactive session", logging.LogOptions{Params: map[string]any{"provider": prov.Name, "model": cfg.Current.Model, "project_hex": hex}})
	rl, err := readline.NewEx(&readline.Config{
		Prompt: termcolor.WrapUser("You: "),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "readline init failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	defer rl.Close()

	sess := &chatstore.Session{
		ID:                     "",
		Title:                  "",
		CreatedAt:              time.Now(),
		LastMessageAt:          time.Now(),
		Messages:               nil,
		CheckpointLast:         -1,
		CheckpointCP0:          true,
		CheckpointBranchSuffix: "",
		ForkChildCount:         nil,
		MainOrphans:            nil,
		LastCommitOID:          "",
	}
	rt := agent.NewRuntime(rl, cfg, prov, hex, root, sess)
	rt.InitMCP(ctx)
	defer rt.Close()
	if len(os.Args) >= 4 && os.Args[1] == "temp" && os.Args[2] == "exec" {
		rt.EphemeralSession = true
		prompt := strings.TrimSpace(strings.Join(os.Args[3:], " "))
		if prompt == "" {
			fmt.Fprintln(os.Stderr, `usage: solomon temp exec <prompt>`)
			logging.Log(logging.WARNING_LOG_LEVEL, "temp exec: missing prompt")
			os.Exit(1)
		}
		logging.Log(logging.INFO_LOG_LEVEL, "one-shot temp exec session")
		if err := rt.RunPromptOnce(ctx, prompt); err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "run prompt failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "exec" {
		prompt := strings.TrimSpace(strings.Join(os.Args[2:], " "))
		if prompt == "" {
			fmt.Fprintln(os.Stderr, `usage: solomon exec <prompt>  (shell quotes grouping text, not passed to Solomon)`)
			logging.Log(logging.WARNING_LOG_LEVEL, "exec: missing prompt")
			os.Exit(1)
		}
		logging.Log(logging.INFO_LOG_LEVEL, "one-shot exec session")
		if err := rt.RunPromptOnce(ctx, prompt); err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "run prompt failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
		return
	}
	if err := rt.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "repl run failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
}
