package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/sandbox/shellpolicy"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestOrchestrateShellRejectsInlinePrograms(t *testing.T) {
	commands := []string{
		`python3 -c 'print(1)'`,
		`/usr/bin/python3.13 -Ic 'print(1)'`,
		`python3 -`,
		`py -3 -c 'print(1)'`,
		`printf 'console.log(1)' | node --input-type=module`,
		`printf 'print(1)' | python3`,
		"python3 <<'PY'\nprint(1)\nPY",
		`node -e 'console.log(1)'`,
		`node --eval='console.log(1)'`,
		`node -p 'JSON.stringify({})'`,
		`node /dev/stdin`,
		`bun -e 'console.log(1)'`,
		`deno eval 'console.log(1)'`,
		`ruby -e 'puts 1'`,
		`perl -pi -e 's/a/b/' file.txt`,
		`lua -e 'print(1)'`,
		`php -r 'echo 1;'`,
		`awk '{print $1}' file.txt`,
		`jq '.value' file.json`,
		`pwsh -NoProfile -Command '1 + 1'`,
		`powershell.exe -EncodedCommand YQ==`,
		`env NAME=value python3 -c 'print(1)'`,
		`sudo -n -u root python3 -c 'print(1)'`,
		`timeout -k 1 5 node -e 'console.log(1)'`,
		`uv run python3 -c 'print(1)'`,
		`npx node -e 'console.log(1)'`,
		`npx -y node -e 'console.log(1)'`,
		`npm exec -- node -e 'console.log(1)'`,
		`env -S "python3 -c 'print(1)'"`,
		`ssh -o BatchMode=yes host "python3 -c 'print(1)'"`,
		`git status; python3 -c 'print(1)'`,
		`git status && node -e 'console.log(1)'`,
		`if true; then python3 -c 'print(1)'; fi`,
		`py'thon3' -c 'print(1)'`,
		"py\\\nthon3 -c 'print(1)'",
		`bash -lc "python3 -c 'print(1)'"`,
		`printf '%s' "$(python3 -c 'print(1)')"`,
		"printf '%s' `python3 -c 'print(1)'`",
		`$runner -c 'print(1)'`,
		`eval "python3 -c 'print(1)'"`,
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if err := shellpolicy.Validate(command); err == nil {
				t.Fatalf("inline program was accepted: %s", command)
			}
		})
	}
}

func TestOrchestrateShellAllowsHostCommandsAndProjectRunners(t *testing.T) {
	commands := []string{
		`git status --short`,
		`go test ./test -count=1`,
		`npm run build`,
		`node --test test/example.test.js`,
		`node --test`,
		`node --check src/example.js`,
		`python3 -m pytest test`,
		`python3 -m unittest discover`,
		`python3 scripts/check.py`,
		`node scripts/check.js`,
		`python3 --version`,
		`node --version`,
		`command -v python3`,
		`env NAME="$HOME" go test ./test`,
		`rg -n 'python3 -c|node -e' docs`,
		`printf '%s' 'python3 -c print(1)'`,
		`printf '%s' '$(python3 -c print(1))'`,
		`git status && go test ./test`,
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if err := shellpolicy.Validate(command); err != nil {
				t.Fatalf("host command was rejected: %v", err)
			}
		})
	}
}

func TestOrchestrateShellRejectsBeforeExecution(t *testing.T) {
	dir := t.TempDir()
	activated := false
	env := &tools.Env{
		ProjRoot: dir, AllowDeferredTools: true, ParentToolName: "orchestrate",
		ActivateInstructionsFromShellCommand: func(string) { activated = true },
	}
	args, err := json.Marshal(map[string]string{
		"command": `printf executed > marker; python3 -c 'print(1)'`,
		"intent":  "verify the inline program guard",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tools.Exec(context.Background(), env, "agent", tooling.Invocation{Name: "shell", Args: args})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected policy rejection, got %v", err)
	}
	if activated {
		t.Fatal("rejected shell command activated instructions")
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
		t.Fatalf("rejected command executed: %v", err)
	}
}

func TestOrchestrateDiscoveryRequiresGoLogic(t *testing.T) {
	result, err := tools.Exec(context.Background(), &tools.Env{}, "agent", tooling.Invocation{
		Name: "searchTools", Args: json.RawMessage(`{"query":"shell","intent":"inspect the Go orchestration contract"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "Orchestration logic must be Go only") {
		t.Fatalf("discovery omitted the Go-only contract: %s", encoded)
	}
}
