package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func TestTranscriptsUseEachMessagesRecordedTarget(t *testing.T) {
	messages := []chatstore.Message{
		{Role: "assistant", Provider: "provider-a", Model: "model-a", Content: "first answer"},
		{Role: "assistant", Provider: "provider-b", Model: "model-b", Content: "second answer"},
		{Role: "assistant", Content: "legacy answer"},
	}
	for _, format := range []string{"terminal", "markdown"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			if format == "terminal" {
				commands.WriteLabeledTranscript(&out, messages, "current-model", false)
			} else {
				meta := commands.MarkdownExportMetaForTest("Recorded targets", "", "current-model")
				if err := commands.WriteMarkdownExportForTest(&out, meta, &chatstore.Session{Messages: messages}, false); err != nil {
					t.Fatal(err)
				}
			}
			for _, want := range []string{"model-a (provider-a)", "model-b (provider-b)", "current-model"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q in transcript: %s", want, out.String())
				}
			}
		})
	}
}
