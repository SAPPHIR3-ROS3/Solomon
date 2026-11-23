package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"path/filepath"
	"strings"
	"testing"

	agenttools "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestPDFReadingAndSearch(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	stream := "BT /F1 10 Tf 15 150 Td (Alpha starts. Beta follows. Gamma arrives. Quartz marks the overlap.) Tj " +
		"0 -14 Td (Epsilon follows. Zeta appears. Eta finishes.) Tj ET"
	writePDFObjects(t, filepath.Join(dir, "text.pdf"), []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 2 /Kids [3 0 R 4 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 420 200] /Resources << /Font << /F1 6 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 420 200] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	})
	env := &agenttools.Env{ProjRoot: dir}
	call := func(name, args string) any {
		t.Helper()
		result, err := execDeferredToolForTest(env, tooling.Invocation{Name: name, Args: json.RawMessage(args)})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	result := call("readPDFPage", `{"path":"text.pdf","page":1,"includeImage":true,"intent":"Read rendered page"}`)
	clean, attachments := images.TakeToolAttachments(result)
	pages := clean.(map[string]any)["pages"].([]pdfinspect.PageText)
	if len(pages) != 1 || !strings.Contains(pages[0].Text, "Quartz") || !pages[0].Image || len(attachments) != 1 {
		t.Fatalf("missing text or image: %+v, attachments %d", pages, len(attachments))
	}
	rendered, err := jpeg.Decode(bytes.NewReader(attachments[0].Data))
	if err != nil {
		t.Fatalf("invalid rendered attachment: %v", err)
	}
	visibleText := false
	for y := rendered.Bounds().Min.Y; y < rendered.Bounds().Max.Y && !visibleText; y++ {
		for x := rendered.Bounds().Min.X; x < rendered.Bounds().Max.X; x++ {
			r, _, _, _ := rendered.At(x, y).RGBA()
			if r < 32768 {
				visibleText = true
				break
			}
		}
	}
	if !visibleText {
		t.Fatal("rendered attachment omitted the visible text")
	}
	result = call("readPDFPages", `{"path":"text.pdf","firstPage":1,"lastPage":2,"intent":"Read native text"}`)
	clean, attachments = images.TakeToolAttachments(result)
	pages = clean.(map[string]any)["pages"].([]pdfinspect.PageText)
	if len(pages) != 2 || pages[1].Number != 2 || pages[1].HasText || len(attachments) != 0 {
		t.Fatalf("invalid text-only response: %+v, attachments %d", pages, len(attachments))
	}
	for _, query := range []string{`"Quartz"`, `"Qaurtz","fuzzy":true`} {
		result := call("searchInPDF", `{"path":"text.pdf","query":`+query+`,"intent":"Find overlap"}`).(pdfinspect.SearchResult)
		if result.Pages != 2 || result.PagesWithText != 1 || len(result.Hits) != 2 {
			t.Fatalf("invalid search response for %s: %+v", query, result)
		}
		for _, hit := range result.Hits {
			if hit.Page != 1 || !strings.Contains(hit.Text, "Quartz") ||
				(hit.FirstSentence != 1 || hit.LastSentence != 4) && (hit.FirstSentence != 4 || hit.LastSentence != 7) {
				t.Fatalf("unexpected block: %+v", hit)
			}
		}
		if result.Hits[0].FirstSentence == result.Hits[1].FirstSentence {
			t.Fatalf("expected distinct overlapping blocks: %+v", result.Hits)
		}
	}
}
