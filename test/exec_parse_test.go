package test

import (
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/cievents"
)

func TestParseExecCLIArgsFlagsAnyOrderPromptLast(t *testing.T) {
	t.Parallel()
	o, err := cievents.ParseExecCLIArgs([]string{"--jsonl", "--no-color", "fix", "the", "tests"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.JSONL || !o.NoColor || o.Prompt != "fix the tests" {
		t.Fatalf("got %+v", o)
	}
}

func TestParseExecCLIArgsFlagAfterPromptRejected(t *testing.T) {
	t.Parallel()
	_, err := cievents.ParseExecCLIArgs([]string{"fix", "--jsonl"})
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func TestParseExecCLIArgsEnvFile(t *testing.T) {
	t.Parallel()
	o, err := cievents.ParseExecCLIArgs([]string{"--env-file", "/tmp/x.env", "--json", "task"})
	if err != nil {
		t.Fatal(err)
	}
	if o.EnvFile != "/tmp/x.env" || !o.JSON || o.Prompt != "task" {
		t.Fatalf("%+v", o)
	}
}
