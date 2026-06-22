package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/multiline"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	cursorint "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/integrations/cursor"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/providersetup"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/prompt"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/termcolor"
	sandboxworker "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/worker"

)

func expandPathArg(raw string) string {
	if raw == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return raw
	}
	if strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, "~\\") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, raw[2:])
		}
	}
	return raw
}

func resolveREPLWorkingDir(args []string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if len(args) < 2 {
		return home, nil
	}
	raw := expandPathArg(args[1])
	abs, err := filepath.Abs(filepath.Clean(raw))
	if err != nil {
		return home, nil
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return home, nil
	}
	return abs, nil
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "version" {
		commands.WriteVersion(os.Stdout)
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "init" {
		if err := config.EnsureDefaultFile(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "sandbox-worker" {
		sandboxworker.Main()
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "templates" && os.Args[2] == "install" {
		runTemplatesInstall()
		return
	}
	ctx := context.Background()
	lroot, err := paths.SolomonHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, configExisted, err := config.LoadStartup()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logging.LogInit(logging.INFO_LOG_LEVEL)
	if cfg != nil && cfg.LogLevel != "" {
		if lvl, err := logging.ParseLevel(cfg.LogLevel); err == nil {
			_ = logging.SetGlobalLevel(lvl)
		}
	}
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout})
	if err := logging.Configure(logging.Config{
		Dir: filepath.Join(lroot, "logs"), WriteConsole: false, WriteFile: true, Retention: 7,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "configure logging failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	logging.Log(logging.INFO_LOG_LEVEL, "Solomon starting")
	if kind, rest := detectExecSubcommand(os.Args); kind != execNone {
		runExecCLI(ctx, kind, rest, cfg)
		return
	}
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
	rl, readLine, rlErr := agentruntime.NewREPLReadline(termcolor.WrapUserReadline("You: "))
	if rlErr != nil {
		fmt.Fprintln(os.Stderr, rlErr)
		logging.Log(logging.ERROR_LOG_LEVEL, "readline init failed", logging.LogOptions{Params: map[string]any{"err": rlErr.Error()}})
		os.Exit(1)
	}
	if rl != nil {
		defer rl.Close()
	}
	setupIO := config.PromptIO{Stdin: os.Stdin, Out: os.Stdout, ReadLine: readLine}
	if err := providersetup.RunInitialSetup(setupIO, os.Stderr, cfg, configExisted); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "initial setup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	if err := prompt.StartupTemplates(cfg, os.Stdout, readLine); err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "prompt template startup failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	config.WriteConfigSetupWarning(os.Stderr, cfg)
	var prov *config.Provider
	if !config.NeedsOnboard(cfg) {
		prov, err = config.ResolveProvider(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			logging.Log(logging.ERROR_LOG_LEVEL, "resolve provider failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
			os.Exit(1)
		}
	}
	wd, err := resolveREPLWorkingDir(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "resolve repl working directory failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	root, hex, err := project.Resolve(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "resolve project failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
	sessParams := map[string]any{"model": cfg.Current.Model, "project_hex": hex, "workspace": root}
	if prov != nil {
		sessParams["provider"] = prov.Name
	}
	logging.Log(logging.INFO_LOG_LEVEL, "interactive session", logging.LogOptions{Params: sessParams})
	cursorint.KickSidecarIfConfigured(ctx, cfg, root, cursorint.DiscardBootstrap{})
	if rl == nil {
		var err2 error
		rl, _, err2 = agentruntime.NewREPLReadline(termcolor.WrapUserReadline("You: "))
		if err2 != nil {
			fmt.Fprintln(os.Stderr, err2)
			logging.Log(logging.ERROR_LOG_LEVEL, "readline init failed", logging.LogOptions{Params: map[string]any{"err": err2.Error()}})
			os.Exit(1)
		}
		if rl == nil {
			fmt.Fprintln(os.Stderr, "interactive session requires a terminal")
			os.Exit(1)
		}
		defer rl.Close()
	}

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
		Branches:               nil,
		LastCommitOID:          "",
	}
	multiline.EnsureCookedTTY()
	rt := agentruntime.NewRuntime(rl, cfg, prov, hex, root, sess)
	defer rt.Close()
	if err := rt.Run(ctx); err != nil {
		if errors.Is(err, agentruntime.ErrRestartSolomon) {
			multiline.EnsureCookedTTY()
			return
		}
		fmt.Fprintln(os.Stderr, err)
		logging.Log(logging.ERROR_LOG_LEVEL, "repl run failed", logging.LogOptions{Params: map[string]any{"err": err.Error()}})
		os.Exit(1)
	}
}

func runTemplatesInstall() {
	if err := config.EnsureDefaultFile(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := config.LoadOptional()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var readLine func(string) (string, error)
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		readLine = func(p string) (string, error) {
			fmt.Fprint(os.Stdout, p)
			line, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return "", err
			}
			return strings.TrimRight(line, "\r\n"), nil
		}
	}
	if err := prompt.InstallTemplates(cfg, os.Stdout, readLine); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
