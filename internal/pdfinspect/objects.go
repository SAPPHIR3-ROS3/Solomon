package pdfinspect

import (
	"fmt"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

func (doc document) visitObjects(page requests.Page, visit func(references.FPDF_PAGEOBJECT, enums.FPDF_PAGEOBJ) error) error {
	count, err := doc.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: page})
	if err != nil {
		return err
	}
	remaining := 200000
	var walk func(references.FPDF_PAGEOBJECT, int) error
	walk = func(object references.FPDF_PAGEOBJECT, depth int) error {
		remaining--
		if depth > 64 || remaining < 0 {
			return fmt.Errorf("PDF page object limits exceeded")
		}
		kind, err := doc.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: object})
		if err != nil {
			return err
		}
		if err := visit(object, kind.Type); err != nil {
			return err
		}
		if kind.Type != enums.FPDF_PAGEOBJ_FORM {
			return nil
		}
		children, err := doc.FPDFFormObj_CountObjects(&requests.FPDFFormObj_CountObjects{PageObject: object})
		if err != nil {
			return err
		}
		for i := 0; i < children.Count; i++ {
			child, err := doc.FPDFFormObj_GetObject(&requests.FPDFFormObj_GetObject{PageObject: object, Index: uint64(i)})
			if err != nil {
				return err
			}
			if err := walk(child.PageObject, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for i := 0; i < count.Count; i++ {
		res, err := doc.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: page, Index: i})
		if err != nil {
			return err
		}
		if err := walk(res.PageObject, 0); err != nil {
			return err
		}
	}
	return nil
}
