package pdfinspect

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
)

type PageText struct {
	Number  int    `json:"page"`
	Text    string `json:"text"`
	HasText bool   `json:"has_text"`
	Image   bool   `json:"image_attached,omitempty"`
}

func ReadPages(ctx context.Context, path string, first, last int, includeImage bool) ([]PageText, [][]byte, bool, error) {
	limit := 25
	if includeImage {
		limit = 5
	}
	if first < 1 || last < first || last-first >= limit {
		return nil, nil, false, fmt.Errorf("expected a valid page range of at most %d pages", limit)
	}
	return readPages(ctx, path, first, last, includeImage, 2<<20)
}

func readPages(ctx context.Context, path string, first, last int, includeImage bool, textLimit int) ([]PageText, [][]byte, bool, error) {
	data, err := loadPDF(path)
	if err != nil {
		return nil, nil, false, err
	}
	engine, ctx, release, err := acquire(ctx, 30*time.Second)
	if err != nil {
		return nil, nil, false, err
	}
	defer release()
	doc, err := openDocument(engine, data)
	if err != nil {
		return nil, nil, false, err
	}
	defer doc.close()
	if last == 0 {
		last = doc.pageCount
	}
	if last > doc.pageCount {
		return nil, nil, false, fmt.Errorf("requested pages %d–%d exceed document length (%d pages)", first, last, doc.pageCount)
	}
	pages := make([]PageText, 0, min(last-first+1, 1000))
	var renders [][]byte
	for number := first; number <= last; number++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, false, err
		}
		page := doc.page(number)
		value, err := doc.GetPageText(&requests.GetPageText{Page: page})
		if err != nil {
			return nil, nil, false, fmt.Errorf("read page %d: %w", number, err)
		}
		content := strings.TrimSpace(strings.ReplaceAll(value.Text, "\r\n", "\n"))
		textLimit -= len(content)
		if textLimit < 0 {
			return nil, nil, false, fmt.Errorf("PDF text exceeds extraction limit")
		}
		if includeImage {
			render, err := renderPage(doc, page)
			if err != nil {
				return nil, nil, false, fmt.Errorf("render page %d: %w", number, err)
			}
			renders = append(renders, render)
		}
		pages = append(pages, PageText{Number: number, Text: content, HasText: content != "", Image: includeImage})
	}
	warning, err := doc.FPDF_DocumentHasValidCrossReferenceTable(&requests.FPDF_DocumentHasValidCrossReferenceTable{Document: doc.ref})
	return pages, renders, err == nil && !warning.DocumentHasValidCrossReferenceTable, nil
}

func renderPage(doc document, page requests.Page) ([]byte, error) {
	output, err := doc.RenderPageInPixels(&requests.RenderPageInPixels{
		Page: page, Width: 1600, Height: 1600, RenderFlags: enums.FPDF_RENDER_FLAG_ANNOT,
	})
	if err != nil {
		return nil, err
	}
	defer output.Cleanup()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, output.Result.RenderedImage, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	if data.Len() > 5<<20 {
		return nil, fmt.Errorf("rendered JPEG exceeds 5 MiB")
	}
	return data.Bytes(), nil
}
