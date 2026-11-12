package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed fixtures/editor-images.json
var archivedChat []byte

func sampleMessages() []message {
	var fixture struct{ Entries []struct{ Role, Text string } }
	if err := json.Unmarshal(archivedChat, &fixture); err != nil {
		return []message{{"ERROR", "Fixture error\n" + err.Error()}}
	}
	result := make([]message, 0, len(fixture.Entries))
	for _, entry := range fixture.Entries {
		result = append(result, message{entry.Role, entry.Text})
	}
	return result
}

func galleryMessages() []message {
	return []message{
		{"SOURCE", "Artifact gallery\nThese additional states demonstrate Solomon UI components. They are not actions or results from the reference chat. No commands are executed."},
		{"YOU", "Show all the UI artifacts in one navigable thread."},
		{"SOLOMON", "This gallery complements the real archived chat. Code, output, diffs, media and session controls share Solomon's role colors and gold turn separators."},
		{"REASONING", "Decision summary\nUse the existing binary file endpoint. Keep image tabs read-only. Preserve text editor autosave. This is a short design summary, not hidden model reasoning."},
		{"PLAN", "Plan\n[x] Inspect file transport\n[x] Render image tabs in read-only mode\n[x] Add regression tests\n[ ] Review terminal artifact rendering"},
		{"TOOL", "searchTools · discover file and editor operations"},
		{"SEARCH", "searchTools\nread image files and editor components\nResult: readFile · Read a workspace file"},
		{"TOOL", "orchestrate · inspect image preview"},
		{"CODE", "Go · orchestrate\npackage main\n\nimport (\n    \"fmt\"\n    \"sdk\"\n)\n\nfunc main() {\n    source, err := sdk.ReadFile(\n        \"gui/src/editor/ImagePreview.tsx\",\n        \"Inspect the image preview\",\n    )\n    if err != nil {\n        fmt.Println(err)\n        return\n    }\n    fmt.Printf(\"Preview source: %d bytes\\n\", len(source))\n}"},
		{"OUTPUT", "Tool output\nPreview source: 2048 bytes\nExit status: 0\nDuration: 128 ms"},
		{"DIFF", "gui/src/editor/EditorPage.tsx · diff\n@@ image tab rendering @@\n- return <CodeEditor file={file} />;\n+ if (isImageFile(file.path)) {\n+   return <ImagePreview file={file} />;\n+ }\n+ return <CodeEditor file={file} />;"},
		{"MEDIA", "Actual image attachment\nicon.png"},
		{"MARKDOWN", "Markdown document\n# Image preview\n\nImages open in a read-only tab.\n\n| Format | Preview | Save |\n| --- | --- | --- |\n| PNG    | yes     | no   |\n| JPEG   | yes     | no   |\n| SVG    | yes     | no   |\n\n- Preserve aspect ratio\n- Show dimensions\n- Recover from invalid images"},
		{"SUBAGENT", "Subagent\n● editor-regressions  completed\n  Model: gpt-6.1-sol\n  Reasoning: high\n  Fast: true\n  Task: verify image tabs do not trigger autosave\n  Result: preview checks passed\n  Subchat: mock-editor-regressions\n○ accessibility-review  queued"},
		{"ERROR", "Tool failure\nImage decode failed: unsupported header\nThe file was not modified.\nRetry: inspect MIME type and original bytes."},
		{"PERMISSION", "Permission request\nAction: read a file outside the selected workspace\nPath: /example/reference.png\n[a] Allow once   [d] Deny\nThis screen does not request or grant access."},
		{"SHELL", "Shell session\n$ go test ./test\nok  example/test  0.214s\n$ git diff --stat\n EditorPage.tsx | 6 +++++-\n 1 file changed, 5 insertions(+), 1 deletion(-)"},
		{"BTW", "/btw · side question\nWhy are images read-only?\nBinary content must not pass through the text autosave path.\nThis answer is presented separately from the main task."},
		{"CHECKPOINT", "Checkpoint\n#0042 · main · image preview integration\n2 files changed · +24 / -6\n/goto would restore this checkpoint in the live app.\nNo restore is performed by this mockup."},
		{"NOTICE", "Notification\nTask completed | No files were changed while displaying this gallery."},
		{"USAGE", "Usage\nContext 18,420 / 128,000 · 14%\nInput 1,240 · Cached 8,120 · Output 684\nFirst token 0.42 s · Output 76 tok/s · Turn 9.4 s"},
		{"SOLOMON", "The gallery is complete. Use Home/End, PgUp/PgDown or the mouse wheel to inspect it. Alt+Up/Down moves between artifacts. Ctrl+D folds or expands block bodies."},
	}
}

func imageMessages() []message {
	return []message{{"YOU", "Show the actual Solomon image."}, {"MEDIA", "Actual image attachment\nicon.png"}, {"SOLOMON", "The PNG is decoded and rendered as real image pixels in terminal cells. The embedded file is the original workspace icon.png, not a text placeholder."}}
}

func presentationMessages(kind string) []message {
	switch kind {
	case "search":
		for _, entry := range sampleMessages() {
			if entry.role != "TOOLCALL" {
				continue
			}
			var tool recordedTool
			if json.Unmarshal([]byte(entry.text), &tool) == nil && tool.Name == "searchTools" {
				return []message{entry}
			}
		}
	case "wrapped_strings":
		return []message{{"CODE", "Go\npackage main\nfunc main() {\n value := \"" + strings.Repeat("x", 360) + "\"\n raw := `RAW-FIRST\nRAW-SECOND`\n _,_ = value,raw\n}"}}
	case "tool_call":
		var selected message
		best := int(^uint(0) >> 1)
		for _, entry := range sampleMessages() {
			if entry.role != "TOOLCALL" {
				continue
			}
			var tool recordedTool
			if json.Unmarshal([]byte(entry.text), &tool) != nil || tool.Name != "orchestrate" || tool.Result == "" {
				continue
			}
			args := decodeResult(string(tool.Arguments))
			source, _ := args["source"].(string)
			if source == "" {
				continue
			}
			size := len(source) + len(resultOutput(tool.Result))
			if size < best {
				selected = entry
				best = size
			}
		}
		if selected.role != "" {
			return []message{selected}
		}
	case "code":
		for _, entry := range galleryMessages() {
			if entry.role == "CODE" {
				return []message{entry}
			}
		}
	case "user":
		return []message{{"YOU", "User messages keep the complete text inside a gold frame."}}
	}
	return nil
}

func artifactTitle(title string) string {
	for _, replacement := range [][2]string{{" · illustrative, not archived events", ""}, {" · illustrative diff", " · diff"}, {" · illustrative side question", " · side question"}, {" · illustrative values", ""}, {" · illustrative", ""}} {
		title = strings.ReplaceAll(title, replacement[0], replacement[1])
	}
	return title
}
