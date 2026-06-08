package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/runtime/repl/shelllex"
)

func TestBuiltinsForShell_powershell(t *testing.T) {
	m := shelllex.BuiltinsForShell(`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`)
	if _, ok := m["ls"]; !ok {
		t.Fatal("powershell builtins should include ls")
	}
	if _, ok := m["dir"]; !ok {
		t.Fatal("powershell builtins should include dir")
	}
}

func TestBuiltinsForShell_cmd(t *testing.T) {
	m := shelllex.BuiltinsForShell(`C:\Windows\System32\cmd.exe`)
	if _, ok := m["dir"]; !ok {
		t.Fatal("cmd builtins should include dir")
	}
	if _, ok := m["ls"]; ok {
		t.Fatal("cmd builtins should not include ls")
	}
}

func TestBuiltinsForShell_pwsh(t *testing.T) {
	m := shelllex.BuiltinsForShell(`C:\Program Files\PowerShell\7\pwsh.exe`)
	if _, ok := m["ls"]; !ok {
		t.Fatal("pwsh builtins should include ls")
	}
}
