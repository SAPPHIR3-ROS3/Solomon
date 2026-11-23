package pdfinspect

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

const maxWASMMemoryPages = 8192

var (
	pdfPool      pdfium.Pool
	pdfPoolErr   error
	pdfPoolReady = make(chan struct{})
)

var startPDFPool = sync.OnceFunc(func() {
	go func() {
		defer close(pdfPoolReady)
		pdfPool, pdfPoolErr = webassembly.Init(webassembly.Config{
			MaxTotal: 2,
			FSConfig: wazero.NewFSConfig(),
			RuntimeConfig: wazero.NewRuntimeConfig().
				WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling).
				WithMemoryLimitPages(maxWASMMemoryPages).
				WithCloseOnContextDone(true),
			Stdout: io.Discard,
			Stderr: io.Discard,
		})
	}()
})

func acquire(ctx context.Context, timeout time.Duration) (pdfium.Pdfium, context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	startPDFPool()
	select {
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	case <-pdfPoolReady:
	}
	if pdfPoolErr != nil {
		return nil, nil, nil, fmt.Errorf("initialize embedded PDFium: %w", pdfPoolErr)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	worker, err := pdfPool.GetInstanceWithContext(ctx)
	if err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("acquire PDFium worker: %w", err)
	}
	killed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(killed)
		_ = worker.Kill()
	})
	return worker, ctx, func() {
		defer cancel()
		if stop() {
			_ = worker.Close()
			return
		}
		<-killed
	}, nil
}

type document struct {
	pdfium.Pdfium
	ref       references.FPDF_DOCUMENT
	pageCount int
}

func openDocument(engine pdfium.Pdfium, data []byte) (document, error) {
	opened, err := engine.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return document{}, fmt.Errorf("open PDF: %w", err)
	}
	doc := document{Pdfium: engine, ref: opened.Document}
	count, err := engine.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.ref})
	if err != nil || count.PageCount < 1 {
		doc.close()
		if err != nil {
			return document{}, fmt.Errorf("read PDF page count: %w", err)
		}
		return document{}, fmt.Errorf("PDF has no pages")
	}
	doc.pageCount = count.PageCount
	return doc, nil
}

func (doc document) close() {
	_, _ = doc.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.ref})
}

func (doc document) page(number int) requests.Page {
	return requests.Page{ByIndex: &requests.PageByIndex{Document: doc.ref, Index: number - 1}}
}

func (doc document) withPage(number int, visit func(requests.Page) error) error {
	page, err := doc.FPDF_LoadPage(&requests.FPDF_LoadPage{Document: doc.ref, Index: number - 1})
	if err != nil {
		return err
	}
	defer doc.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: page.Page})
	return visit(requests.Page{ByReference: &page.Page})
}
