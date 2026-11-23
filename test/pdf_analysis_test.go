package test

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/tools"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/tooling"
)

func writeAnalysisPDF(t *testing.T, directory string) {
	t.Helper()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /OpenAction 5 0 R /Names << /JavaScript << /Names [(startup) 5 0 R] >> >> >>",
		"<< /Type /Pages /Count 2 /Kids [3 0 R 4 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 400 500] /Rotate 90 >>",
		"<< /Type /Action /S /Java#53cript /JS (analysis-fixture) >>",
	}
	var kids strings.Builder
	kids.WriteString("3 0 R 4 0 R")
	for len(objects) < 33 {
		fmt.Fprintf(&kids, " %d 0 R", len(objects)+1)
		objects = append(objects, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] >>")
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Count 30 /Kids [%s] >>", kids.String())
	writePDFObjects(t, filepath.Join(directory, "fixture.pdf"), objects)
}

func writePDFObjects(t *testing.T, path string, objects []string) {
	t.Helper()
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.7\n")
	var offsets []int
	for i, object := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	if err := os.WriteFile(path, pdf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzePDF(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	writeAnalysisPDF(t, dir)
	env := &tools.Env{ProjRoot: dir, AllowDeferredTools: true}
	result, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "analyzePDFFast", Args: json.RawMessage(`{"path":"fixture.pdf","intent":"Inspect PDF"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	report := result.(pdfinspect.Report)
	if report.RawNameHints["JavaScript"] < 1 || report.RawNameHints["open action or destination"] != 1 {
		t.Fatalf("missing structural hints: %+v", report)
	}
	if report.Status != "review" || report.Indicators["javascript_reported_by_parser"] != 1 ||
		report.PageCount != 30 || len(report.Pages) != 25 ||
		report.Pages[0].WidthPoints != 200 || report.Pages[0].Images == nil || report.FontEntriesInRange == nil {
		t.Fatalf("unexpected fast report: %+v", report)
	}
	full, err := tools.Exec(context.Background(), env, "agent", tooling.Invocation{
		Name: "analyzePDFFull", Args: json.RawMessage(`{"path":"fixture.pdf","intent":"Inspect the whole PDF"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	document := full.(pdfinspect.Report)
	if document.Status != "review" || len(document.Pages) != 30 || document.Pages[29].Number != 30 ||
		document.Pages[1].Rotation != 90 || document.Pages[29].Images == nil {
		t.Fatalf("expected statistics for the whole PDF: %+v", document)
	}

}

func TestAnalyzePDFCompressedActions(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "compressed.pdf")
	var file bytes.Buffer
	file.WriteString("%PDF-1.7\n")
	offsets := make([]int, 7)
	write := func(i int, object string) {
		offsets[i] = file.Len()
		fmt.Fprintf(&file, "%d 0 obj\n%s\nendobj\n", i, object)
	}
	write(1, "<< /Type /Catalog /Pages 2 0 R /OpenAction 5 0 R >>")
	write(2, "<< /Type /Pages /Count 1 /Kids [3 0 R] >>")
	write(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 300] >>")
	var compressed bytes.Buffer
	z := zlib.NewWriter(&compressed)
	if _, err := z.Write([]byte("5 0 << /S /JavaScript /JS (" + strings.Repeat("compressed analysis fixture ", 80) + ") >>")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	write(4, fmt.Sprintf("<< /Type /ObjStm /N 1 /First 4 /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", compressed.Len(), compressed.Bytes()))
	offsets[6] = file.Len()
	var xref bytes.Buffer
	for number, offset := range offsets {
		kind, value, index := byte(1), uint32(offset), uint16(0)
		switch number {
		case 0:
			kind, index = 0, 65535
		case 5:
			kind, value = 2, 4
		}
		xref.WriteByte(kind)
		if err := binary.Write(&xref, binary.BigEndian, value); err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(&xref, binary.BigEndian, index); err != nil {
			t.Fatal(err)
		}
	}
	write(6, fmt.Sprintf("<< /Type /XRef /Size 7 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n%s\nendstream", xref.Len(), xref.Bytes()))
	fmt.Fprintf(&file, "startxref\n%d\n%%%%EOF\n", offsets[6])
	if bytes.Contains(file.Bytes(), []byte("/JavaScript")) {
		t.Fatal("fixture contains an uncompressed JavaScript marker")
	}
	if err := os.WriteFile(path, file.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := pdfinspect.Analyze(context.Background(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "review" || report.PageCount != 1 || report.RawNameHints["JavaScript"] != 1 {
		t.Fatalf("compressed JavaScript indicator not reported: %+v", report)
	}
	if report.Indicators["javascript_reported_by_parser"] != 0 {
		t.Fatal("an action found by raw inspection must not be attributed to the parser")
	}
}
