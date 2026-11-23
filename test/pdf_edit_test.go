package test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func TestPDFEditTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	writePDFObjects(t, filepath.Join(dir, "a.pdf"), []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 2 /Kids [3 0 R 4 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 201 300] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 202 300] >>",
	})
	writePDFObjects(t, filepath.Join(dir, "b.pdf"), []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 303 300] >>",
	})
	env := &tools.Env{ProjRoot: dir}
	for _, tc := range []struct {
		tool, output, args string
		widths             []float64
	}{
		{"editPDFPages", "edit.pdf", `{"outputPath":"edit.pdf","sources":[{"path":"a.pdf","pages":[2,1,2]},{"path":"b.pdf"}],"intent":"Reorder pages"}`, []float64{202, 201, 202, 303}},
		{"mergePDFs", "merge.pdf", `{"outputPath":"merge.pdf","paths":["b.pdf","a.pdf"],"intent":"Merge PDFs"}`, []float64{303, 201, 202}},
		{"deletePDFPages", "delete.pdf", `{"path":"a.pdf","outputPath":"delete.pdf","pages":[1],"intent":"Omit page one"}`, []float64{202}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			inv := tooling.Invocation{Name: tc.tool, Args: json.RawMessage(tc.args)}
			out, err := execDeferredToolForTest(env, inv)
			if err != nil {
				t.Fatal(err)
			}
			if count := out.(map[string]any)["page_count"]; count != len(tc.widths) {
				t.Fatalf("unexpected page count: %v", count)
			}
			report, err := pdfinspect.Analyze(context.Background(), filepath.Join(dir, tc.output), 0)
			if err != nil {
				t.Fatal(err)
			}
			var widths []float64
			for _, page := range report.Pages {
				widths = append(widths, page.WidthPoints)
			}
			if !reflect.DeepEqual(widths, tc.widths) {
				t.Fatalf("page order %v, want %v", widths, tc.widths)
			}
			if _, err := execDeferredToolForTest(env, inv); err == nil {
				t.Fatal("tool overwrote an existing output")
			}
		})
	}
}

func TestAnnotatePDF(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	writePDFObjects(t, filepath.Join(dir, "original.pdf"), []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] >>",
	})
	env := &tools.Env{ProjRoot: dir}
	for _, args := range []string{
		`{"path":"original.pdf","outputPath":"note.pdf","page":1,"kind":"note","rect":[10,10,40,40],"text":"Nota già verificata","intent":"Add a note"}`,
		`{"path":"original.pdf","outputPath":"highlight.pdf","page":1,"kind":"highlight","rect":[10,10,40,40],"intent":"Highlight PDF text"}`,
	} {
		inv := tooling.Invocation{Name: "annotatePDF", Args: json.RawMessage(args)}
		out, err := execDeferredToolForTest(env, inv)
		if err != nil {
			t.Fatal(err)
		}
		if count := out.(map[string]any)["page_count"]; count != 1 {
			t.Fatalf("unexpected annotated page count: %v", count)
		}
		if _, err := execDeferredToolForTest(env, inv); err == nil {
			t.Fatal("annotation tool overwrote an existing output")
		}
	}
}
