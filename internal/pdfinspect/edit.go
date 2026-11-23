package pdfinspect

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
)

type PageSource struct {
	Path  string `json:"path"`
	Pages []int  `json:"pages"`
}

func EditPages(ctx context.Context, output string, sources []PageSource, deletePages []int) (int, error) {
	if len(sources) == 0 || len(sources) > 100 {
		return 0, fmt.Errorf("expected 1–100 PDF sources")
	}
	if err := checkPDFDestination(output); err != nil {
		return 0, err
	}
	engine, ctx, release, err := acquire(ctx, 60*time.Second)
	if err != nil {
		return 0, err
	}
	defer release()
	target, err := engine.FPDF_CreateNewDocument(&requests.FPDF_CreateNewDocument{})
	if err != nil {
		return 0, err
	}
	dest := document{Pdfium: engine, ref: target.Document}
	defer dest.close()
	expected, selected := 0, 0
	for _, source := range sources {
		count, explicit, err := importSource(dest, source, deletePages, expected, 1000-selected)
		if err != nil {
			return 0, fmt.Errorf("import %s: %w", source.Path, err)
		}
		expected += count
		selected += explicit
	}
	if err := saveDocument(ctx, dest, output, expected, nil); err != nil {
		return 0, err
	}
	return expected, nil
}

func importSource(dest document, source PageSource, deleted []int, offset, remaining int) (int, int, error) {
	data, err := loadPDF(source.Path)
	if err != nil {
		return 0, 0, err
	}
	doc, err := openDocument(dest.Pdfium, data)
	if err != nil {
		return 0, 0, err
	}
	defer doc.close()
	pages := source.Pages
	if len(deleted) > 0 {
		pages, err = remainingPages(doc.pageCount, deleted)
		if err != nil {
			return 0, 0, err
		}
	}
	count, explicit := doc.pageCount, 0
	var indices []int
	if pages != nil {
		if len(pages) == 0 || len(pages) > remaining {
			return 0, 0, fmt.Errorf("explicit selection must produce between 1 and 1000 page inputs")
		}
		indices = make([]int, len(pages))
		for i, page := range pages {
			if page < 1 || page > doc.pageCount {
				return 0, 0, fmt.Errorf("page %d out of range (document has %d pages)", page, doc.pageCount)
			}
			indices[i] = page - 1
		}
		count, explicit = len(indices), len(indices)
	}
	_, err = dest.FPDF_ImportPagesByIndex(&requests.FPDF_ImportPagesByIndex{
		Source: doc.ref, Destination: dest.ref, PageIndices: indices, Index: offset,
	})
	return count, explicit, err
}

type limitedPDFWriter struct {
	io.Writer
	remaining int
}

func (w *limitedPDFWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		return 0, fmt.Errorf("generated PDF exceeds 128 MiB")
	}
	n, err := w.Writer.Write(data)
	w.remaining -= n
	return n, err
}

func remainingPages(count int, deleted []int) ([]int, error) {
	if count-len(deleted) > 1000 {
		return nil, fmt.Errorf("page deletion supports at most 1000 retained pages")
	}
	deleted = slices.Clone(deleted)
	slices.Sort(deleted)
	deleted = slices.Compact(deleted)
	if deleted[0] < 1 || deleted[len(deleted)-1] > count {
		return nil, fmt.Errorf("page to delete out of range (document has %d pages)", count)
	}
	pages := make([]int, 0, min(1000, count-len(deleted)))
	for page := 1; page <= count; page++ {
		if len(deleted) > 0 && page == deleted[0] {
			deleted = deleted[1:]
			continue
		}
		if len(pages) == 1000 {
			return nil, fmt.Errorf("page deletion supports at most 1000 retained pages")
		}
		pages = append(pages, page)
	}
	return pages, nil
}
