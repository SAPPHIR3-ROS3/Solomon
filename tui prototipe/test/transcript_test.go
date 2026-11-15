package test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTranscriptIncludesSupplementaryArtifacts(t *testing.T) {
	out, err := exec.Command("../laboratory", "--transcript").CombinedOutput()
	if err != nil {
		t.Fatalf("transcript: %v\n%s", err, out)
	}
	for _, label := range []string{"Mermaid · diagram gallery", "01 · Flowchart", "10 · Pie", "15 · Sankey", "22 · Radar", "25 · Agent flow", "Prototype artifacts", "Context compacted", "3 changed files", "worked for 12s", "subagents working", "Subagent chat", "Usage limit reached", "Generation interrupted", "Regenerate", "L2 Subagent"} {
		if !strings.Contains(string(out), label) {
			t.Errorf("missing artifact: %s", label)
		}
	}
}
