package pdfinspect

import (
	"context"
	"fmt"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

func inspect(ctx context.Context, data []byte, pageLimit int, report *Report) {
	engine, ctx, release, err := acquire(ctx, 25*time.Second)
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return
	}
	defer release()
	doc, err := openDocument(engine, data)
	if err != nil {
		report.Issues = append(report.Issues, err.Error())
		return
	}
	defer doc.close()
	report.PageCount = doc.pageCount
	reportDocument(doc, report)
	last := doc.pageCount
	if pageLimit > 0 {
		last = min(pageLimit, last)
	}
	fonts := make(map[string]struct{})
	for number := 1; number <= last; number++ {
		if ctx.Err() != nil {
			report.Issues = append(report.Issues, ctx.Err().Error())
			break
		}
		if err := reportPage(doc, number, fonts, report); err != nil {
			report.Issues = append(report.Issues, fmt.Sprintf("page %d: %v", number, err))
			break
		}
	}
	total := len(fonts)
	report.FontEntriesInRange = &total
}

func reportDocument(doc document, report *Report) {
	if version, err := doc.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{Document: doc.ref}); err == nil {
		report.Properties["PDF version"] = fmt.Sprintf("%d.%d", version.FileVersion/10, version.FileVersion%10)
	}
	if tagged, err := doc.FPDFCatalog_IsTagged(&requests.FPDFCatalog_IsTagged{Document: doc.ref}); err == nil {
		report.Properties["Tagged"] = yesNo(tagged.IsTagged)
	}
	if revision, err := doc.FPDF_GetSecurityHandlerRevision(&requests.FPDF_GetSecurityHandlerRevision{Document: doc.ref}); err == nil {
		report.Properties["Encrypted"] = yesNo(revision.SecurityHandlerRevision >= 0)
	}
	if form, err := doc.FPDF_GetFormType(&requests.FPDF_GetFormType{Document: doc.ref}); err == nil {
		switch form.FormType {
		case enums.FPDF_FORMTYPE_NONE:
			report.Properties["Form"] = "none"
		case enums.FPDF_FORMTYPE_ACRO_FORM:
			report.Properties["Form"] = "AcroForm"
		default:
			report.Properties["Form"] = "XFA"
		}
	}
	if table, err := doc.FPDF_DocumentHasValidCrossReferenceTable(&requests.FPDF_DocumentHasValidCrossReferenceTable{Document: doc.ref}); err == nil {
		report.Properties["Suspects"] = yesNo(!table.DocumentHasValidCrossReferenceTable)
		if !table.DocumentHasValidCrossReferenceTable {
			report.Indicators["suspect_structure_reported_by_parser"] = 1
		}
	}
	if scripts, err := doc.FPDFDoc_GetJavaScriptActionCount(&requests.FPDFDoc_GetJavaScriptActionCount{Document: doc.ref}); err == nil && scripts.JavaScriptActionCount > 0 {
		report.Indicators["javascript_reported_by_parser"] = scripts.JavaScriptActionCount
	}
	if files, err := doc.FPDFDoc_GetAttachmentCount(&requests.FPDFDoc_GetAttachmentCount{Document: doc.ref}); err == nil && files.AttachmentCount > 0 {
		report.Indicators["attachments_reported_by_parser"] = files.AttachmentCount
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func reportPage(doc document, number int, fonts map[string]struct{}, report *Report) error {
	return doc.withPage(number, func(page requests.Page) error {
		size, err := doc.GetPageSize(&requests.GetPageSize{Page: page})
		if err != nil {
			return err
		}
		rotation, err := doc.FPDFPage_GetRotation(&requests.FPDFPage_GetRotation{Page: page})
		if err != nil {
			return err
		}
		images := 0
		err = doc.visitObjects(page, func(object references.FPDF_PAGEOBJECT, kind enums.FPDF_PAGEOBJ) error {
			switch kind {
			case enums.FPDF_PAGEOBJ_IMAGE:
				images++
			case enums.FPDF_PAGEOBJ_TEXT:
				font, err := doc.FPDFTextObj_GetFont(&requests.FPDFTextObj_GetFont{PageObject: object})
				if err == nil {
					if name, err := doc.FPDFFont_GetBaseFontName(&requests.FPDFFont_GetBaseFontName{Font: font.Font}); err == nil {
						fonts[name.BaseFontName] = struct{}{}
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		report.Pages = append(report.Pages, Page{
			Number: number, WidthPoints: size.Width, HeightPoints: size.Height,
			Rotation: int(rotation.PageRotation) * 90, Boxes: reportBoxes(doc, page), Images: &images,
		})
		return reportLinks(doc, page, report)
	})
}

func reportBoxes(doc document, page requests.Page) map[string][4]float64 {
	boxes := make(map[string][4]float64)
	if v, err := doc.FPDFPage_GetMediaBox(&requests.FPDFPage_GetMediaBox{Page: page}); err == nil {
		boxes["MediaBox"] = [4]float64{float64(v.Left), float64(v.Bottom), float64(v.Right), float64(v.Top)}
	}
	if v, err := doc.FPDFPage_GetCropBox(&requests.FPDFPage_GetCropBox{Page: page}); err == nil {
		boxes["CropBox"] = [4]float64{float64(v.Left), float64(v.Bottom), float64(v.Right), float64(v.Top)}
	} else if media, ok := boxes["MediaBox"]; ok {
		boxes["CropBox"] = media
	}
	if v, err := doc.FPDFPage_GetBleedBox(&requests.FPDFPage_GetBleedBox{Page: page}); err == nil {
		boxes["BleedBox"] = [4]float64{float64(v.Left), float64(v.Bottom), float64(v.Right), float64(v.Top)}
	}
	if v, err := doc.FPDFPage_GetTrimBox(&requests.FPDFPage_GetTrimBox{Page: page}); err == nil {
		boxes["TrimBox"] = [4]float64{float64(v.Left), float64(v.Bottom), float64(v.Right), float64(v.Top)}
	}
	if v, err := doc.FPDFPage_GetArtBox(&requests.FPDFPage_GetArtBox{Page: page}); err == nil {
		boxes["ArtBox"] = [4]float64{float64(v.Left), float64(v.Bottom), float64(v.Right), float64(v.Top)}
	}
	if crop, ok := boxes["CropBox"]; ok {
		for _, key := range []string{"BleedBox", "TrimBox", "ArtBox"} {
			if _, ok := boxes[key]; !ok {
				boxes[key] = crop
			}
		}
	}
	return boxes
}

func reportLinks(doc document, page requests.Page, report *Report) error {
	for pos := 0; ; {
		link, err := doc.FPDFLink_Enumerate(&requests.FPDFLink_Enumerate{Page: page, StartPos: pos})
		if err != nil {
			return err
		}
		if link.Link == nil || link.NextStartPos == nil {
			return nil
		}
		if *link.NextStartPos <= pos {
			return fmt.Errorf("PDFium link enumeration did not advance")
		}
		pos = *link.NextStartPos
		action, err := doc.FPDFLink_GetAction(&requests.FPDFLink_GetAction{Link: *link.Link})
		if err != nil {
			return err
		}
		if action.Action == nil {
			continue
		}
		kind, err := doc.FPDFAction_GetType(&requests.FPDFAction_GetType{Action: *action.Action})
		if err != nil {
			return err
		}
		switch kind.Type {
		case enums.FPDF_ACTION_ACTION_URI:
			report.Indicators["url_entries_reported_by_parser"]++
		case enums.FPDF_ACTION_ACTION_LAUNCH:
			report.Indicators["launch_actions_reported_by_parser"]++
		case enums.FPDF_ACTION_ACTION_EMBEDDEDGOTO, enums.FPDF_ACTION_ACTION_REMOTEGOTO:
			report.Indicators["external_destinations_reported_by_parser"]++
		}
	}
}
