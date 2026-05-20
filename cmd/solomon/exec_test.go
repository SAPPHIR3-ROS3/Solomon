package main

import "testing"

func TestParseExecArgsFlagsAnyOrderPromptLast(t *testing.T) {
	t.Parallel()
	o, err := parseExecArgs([]string{"--jsonl", "--no-color", "fix", "the", "tests"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.JSONL || !o.NoColor || o.Prompt != "fix the tests" {
		t.Fatalf("got %+v", o)
	}
}

func TestParseExecArgsFlagAfterPromptRejected(t *testing.T) {
	t.Parallel()
	_, err := parseExecArgs([]string{"fix", "--jsonl"})
	if err == nil {
		t.Fatal("expected usage error")
	}
}

func TestParseExecArgsEnvFile(t *testing.T) {
	t.Parallel()
	o, err := parseExecArgs([]string{"--env-file", "/tmp/x.env", "--json", "task"})
	if err != nil {
		t.Fatal(err)
	}
	if o.EnvFile != "/tmp/x.env" || !o.JSON || o.Prompt != "task" {
		t.Fatalf("%+v", o)
	}
}
