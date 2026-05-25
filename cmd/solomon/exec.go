package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/agent/cievents"
	agentruntime "github.com/SAPPHIR3-ROS3/Solomon/internal/agent/runtime"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/config"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/project"
	"github.com/SAPPHIR3-ROS3/Solomon/internal/termcolor"

	readline "github.com/chzyer/readline"
)

type execKind int

const (
	execNone execKind = iota
	execNormal
	execTemp
)

func detectExecSubcommand(args []string) (execKind, []string) {
	if len(args) >= 2 && args[1] == "exec" {
		return execNormal, args[2:]
	}
	if len(args) >= 4 && args[1] == "temp" && args[2] == "exec" {
		return execTemp, args[3:]
	}
	return execNone, nil
}

func runExecCLI(ctx context.Context, kind execKind, argRest []string) {
	opts, err := cievents.ParseExecCLIArgs(argRest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitExec(cievents.ClassifyExit(err))
	}
	if opts.JSON && opts.JSONL {
		fmt.Fprintln(os.Stderr, "cannot use --json and --jsonl together")
		exitExec(cievents.ExitUsage, "usage")
	}
	machine := opts.JSON || opts.JSONL
	if strings.TrimSpace(opts.Prompt) == "" {
		usage := `usage: solomon exec [--json|--jsonl] [--env-file path] [--no-color] [--fail-on-tool-error] <prompt>`
		if kind == execTemp {
			usage = `usage: solomon temp exec [--json|--jsonl] [--env-file path] [--no-color] [--fail-on-tool-error] <prompt>`
		}
		fmt.Fprintln(os.Stderr, usage)
		exitExec(cievents.ExitUsage, "usage")
	}
	var cfg *config.Root
	var prov *config.Provider
	if machine {
		loaded, loadErr := config.Load()
		if loadErr != nil {
			loaded = nil
		}
		cfg, prov, err = config.ResolveExecConfig(loaded, config.ExecResolveOpts{EnvFile: opts.EnvFile})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitExec(cievents.ExitConfig, "config")
		}
	} else {
		cfg, err = config.LoadOptional()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitExec(cievents.ExitConfig, "config")
		}
		if config.NeedsOnboard(cfg) {
			fmt.Fprintln(os.Stderr, "config not set up; run solomon and use /onboard")
			exitExec(cievents.ExitConfig, "config")
		}
		prov, err = config.ResolveProvider(cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitExec(cievents.ExitConfig, "config")
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitExec(cievents.ExitGeneric, "error")
	}
	root, hex, err := project.Resolve(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitExec(cievents.ExitGeneric, "error")
	}
	_ = root
	if opts.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}
	termcolor.Init(termcolor.InitOptions{Out: os.Stdout, NoColor: opts.NoColor})
	var rl *readline.Instance
	if !machine {
		var err2 error
		rl, _, err2 = agentruntime.NewREPLReadline(termcolor.WrapUserReadline("You: "))
		if err2 != nil {
			fmt.Fprintln(os.Stderr, err2)
			exitExec(cievents.ExitGeneric, "error")
		}
		if rl == nil {
			fmt.Fprintln(os.Stderr, "interactive exec requires a terminal")
			exitExec(cievents.ExitGeneric, "error")
		}
		defer rl.Close()
	}
	sess := &chatstore.Session{
		CreatedAt:      time.Now(),
		LastMessageAt:  time.Now(),
		CheckpointLast: -1,
		CheckpointCP0:  true,
	}
	rt := agentruntime.NewRuntime(rl, cfg, prov, hex, root, sess)
	if kind == execTemp {
		rt.EphemeralSession = true
	}
	if machine {
		if opts.JSONL {
			rt.EventSink = cievents.NewJSONLEmitter(os.Stdout)
		} else {
			rt.EventSink = cievents.NewJSONCollector(os.Stdout)
		}
		rt.FailOnToolError = opts.FailOnToolError
	}
	rt.InitMCP(ctx)
	defer rt.Close()
	logging.Log(logging.INFO_LOG_LEVEL, "one-shot exec session", logging.LogOptions{Params: map[string]any{"machine": machine, "jsonl": opts.JSONL}})
	if err := rt.RunPromptOnce(ctx, opts.Prompt); err != nil {
		if !machine {
			fmt.Fprintln(os.Stderr, err)
		}
		code, _ := cievents.ClassifyExit(err)
		exitExec(code, "")
	}
	exitExec(cievents.ExitOK, "ok")
}

func exitExec(code int, _ string) {
	os.Exit(code)
}
