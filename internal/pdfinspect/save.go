package pdfinspect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/klippa-app/go-pdfium/requests"
)

func checkPDFDestination(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("destination already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func saveDocument(ctx context.Context, doc document, output string, expected int, verify func(document) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(output), ".pdf-edit-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	writer := &limitedPDFWriter{Writer: tmp, remaining: maxFileBytes}
	_, saveErr := doc.FPDF_SaveAsCopy(&requests.FPDF_SaveAsCopy{
		Document: doc.ref, Flags: requests.SaveFlagNoIncremental, FileWriter: writer,
	})
	closeErr := tmp.Close()
	if saveErr != nil {
		return fmt.Errorf("save PDF: %w", saveErr)
	}
	if closeErr != nil {
		return closeErr
	}
	data, err := loadPDF(tmp.Name())
	if err != nil {
		return err
	}
	verified, err := openDocument(doc.Pdfium, data)
	if err != nil {
		return err
	}
	defer verified.close()
	if verified.pageCount != expected {
		return fmt.Errorf("output has %d pages, expected %d", verified.pageCount, expected)
	}
	if verify != nil {
		if err := verify(verified); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Link(tmp.Name(), output); err != nil {
		return err
	}
	return nil
}
