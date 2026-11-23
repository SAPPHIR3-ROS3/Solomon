package pdfinspect

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/structs"
)

type Annotation struct {
	Page int       `json:"page"`
	Kind string    `json:"kind"`
	Rect []float32 `json:"rect"`
	Text string    `json:"text,omitempty"`
}

func Annotate(ctx context.Context, path, output string, annotation Annotation) (int, error) {
	if err := checkPDFDestination(output); err != nil {
		return 0, err
	}
	if annotation.Page < 1 || len(annotation.Rect) != 4 || len(annotation.Text) > 8192 || strings.ContainsRune(annotation.Text, 0) {
		return 0, fmt.Errorf("expected a valid page, four rectangle coordinates, and at most 8192 bytes of annotation text without NUL")
	}
	rect := annotation.Rect
	for _, value := range rect {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("annotation rectangle coordinates must be finite")
		}
	}
	if rect[0] >= rect[2] || rect[1] >= rect[3] {
		return 0, fmt.Errorf("annotation rectangle must be [left, bottom, right, top] with positive area")
	}
	var subtype enums.FPDF_ANNOTATION_SUBTYPE
	switch annotation.Kind {
	case "note":
		if strings.TrimSpace(annotation.Text) == "" {
			return 0, fmt.Errorf("a note requires text")
		}
		subtype = enums.FPDF_ANNOT_SUBTYPE_TEXT
	case "highlight":
		subtype = enums.FPDF_ANNOT_SUBTYPE_HIGHLIGHT
	default:
		return 0, fmt.Errorf("annotation kind must be note or highlight")
	}
	data, err := loadPDF(path)
	if err != nil {
		return 0, err
	}
	engine, ctx, release, err := acquire(ctx, 30*time.Second)
	if err != nil {
		return 0, err
	}
	defer release()
	doc, err := openDocument(engine, data)
	if err != nil {
		return 0, err
	}
	defer doc.close()
	if annotation.Page > doc.pageCount {
		return 0, fmt.Errorf("page %d out of range (document has %d pages)", annotation.Page, doc.pageCount)
	}
	index := 0
	err = doc.withPage(annotation.Page, func(page requests.Page) error {
		count, err := doc.FPDFPage_GetAnnotCount(&requests.FPDFPage_GetAnnotCount{Page: page})
		if err != nil {
			return err
		}
		index = count.Count
		boxes := reportBoxes(doc, page)
		if crop, ok := boxes["CropBox"]; ok && (float64(rect[0]) < crop[0] || float64(rect[1]) < crop[1] || float64(rect[2]) > crop[2] || float64(rect[3]) > crop[3]) {
			return fmt.Errorf("annotation rectangle must be within the page crop box")
		}
		created, err := doc.FPDFPage_CreateAnnot(&requests.FPDFPage_CreateAnnot{Page: page, Subtype: subtype})
		if err != nil {
			return err
		}
		ref := created.Annotation
		defer doc.FPDFPage_CloseAnnot(&requests.FPDFPage_CloseAnnot{Annotation: ref})
		if _, err = doc.FPDFAnnot_SetRect(&requests.FPDFAnnot_SetRect{
			Annotation: ref, Rect: structs.FPDF_FS_RECTF{Left: rect[0], Bottom: rect[1], Right: rect[2], Top: rect[3]},
		}); err != nil {
			return err
		}
		if subtype == enums.FPDF_ANNOT_SUBTYPE_HIGHLIGHT {
			if _, err = doc.FPDFAnnot_AppendAttachmentPoints(&requests.FPDFAnnot_AppendAttachmentPoints{
				Annotation: ref, AttachmentPoints: structs.FPDF_FS_QUADPOINTSF{X1: rect[0], Y1: rect[3], X2: rect[2], Y2: rect[3], X3: rect[0], Y3: rect[1], X4: rect[2], Y4: rect[1]},
			}); err != nil {
				return err
			}
			if _, err = doc.FPDFAnnot_SetColor(&requests.FPDFAnnot_SetColor{Annotation: ref, R: 255, G: 230, B: 64, A: 128}); err != nil {
				return err
			}
		}
		if annotation.Text != "" {
			_, err = doc.FPDFAnnot_SetStringValue(&requests.FPDFAnnot_SetStringValue{Annotation: ref, Key: "Contents", Value: annotation.Text})
		}
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("annotate PDF page %d: %w", annotation.Page, err)
	}
	verify := func(saved document) error {
		return saved.withPage(annotation.Page, func(page requests.Page) error {
			found, err := saved.FPDFPage_GetAnnot(&requests.FPDFPage_GetAnnot{Page: page, Index: index})
			if err != nil {
				return fmt.Errorf("reopen saved annotation: %w", err)
			}
			defer saved.FPDFPage_CloseAnnot(&requests.FPDFPage_CloseAnnot{Annotation: found.Annotation})
			kind, err := saved.FPDFAnnot_GetSubtype(&requests.FPDFAnnot_GetSubtype{Annotation: found.Annotation})
			if err != nil {
				return err
			}
			if kind.Subtype != subtype {
				return fmt.Errorf("saved annotation type does not match request")
			}
			if annotation.Text != "" {
				text, err := saved.FPDFAnnot_GetStringValue(&requests.FPDFAnnot_GetStringValue{Annotation: found.Annotation, Key: "Contents"})
				if err != nil {
					return err
				}
				if text.Value != annotation.Text {
					return fmt.Errorf("saved annotation text does not match request")
				}
			}
			if subtype == enums.FPDF_ANNOT_SUBTYPE_HIGHLIGHT {
				points, err := saved.FPDFAnnot_CountAttachmentPoints(&requests.FPDFAnnot_CountAttachmentPoints{Annotation: found.Annotation})
				if err != nil {
					return err
				}
				if points.Count != 1 {
					return fmt.Errorf("saved highlight rectangle is missing")
				}
			}
			return nil
		})
	}
	if err := saveDocument(ctx, doc, output, doc.pageCount, verify); err != nil {
		return 0, err
	}
	return doc.pageCount, nil
}
