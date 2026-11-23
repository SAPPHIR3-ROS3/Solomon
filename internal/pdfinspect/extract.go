package pdfinspect

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

type ExtractedImage struct {
	Path  string `json:"path"`
	Page  int    `json:"page"`
	Bytes int64  `json:"size_bytes"`
}

func ExtractImages(ctx context.Context, source, destination string, first, last int) ([]ExtractedImage, error) {
	if first < 0 || last < 0 || (first > 0 && last > 0 && last < first) {
		return nil, fmt.Errorf("expected 1-based inclusive page bounds or 0 for an open bound")
	}
	if _, err := os.Lstat(destination); err == nil {
		return nil, fmt.Errorf("destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	data, err := loadPDF(source)
	if err != nil {
		return nil, err
	}
	engine, ctx, release, err := acquire(ctx, 30*time.Second)
	if err != nil {
		return nil, err
	}
	defer release()
	doc, err := openDocument(engine, data)
	if err != nil {
		return nil, err
	}
	defer doc.close()
	first = max(first, 1)
	if last == 0 {
		last = doc.pageCount
	}
	if first > doc.pageCount || last > doc.pageCount {
		return nil, fmt.Errorf("page range exceeds document length (%d pages)", doc.pageCount)
	}
	work, err := os.MkdirTemp(filepath.Dir(destination), ".pdf-images-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	result := []ExtractedImage{}
	var total int64
	for number := first; number <= last; number++ {
		err = doc.withPage(number, func(page requests.Page) error {
			return doc.visitObjects(page, func(object references.FPDF_PAGEOBJECT, kind enums.FPDF_PAGEOBJ) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if kind != enums.FPDF_PAGEOBJ_IMAGE {
					return nil
				}
				if len(result) >= 500 {
					return fmt.Errorf("extract at most 500 images per call")
				}
				encoded, ext, err := extractImage(doc, object)
				if err != nil {
					return err
				}
				total += int64(len(encoded))
				if total > 256<<20 {
					return fmt.Errorf("extracted images exceed 256 MiB; use a smaller page range")
				}
				name := fmt.Sprintf("image-%03d-%03d.%s", number, len(result), ext)
				if err := os.WriteFile(filepath.Join(work, name), encoded, 0600); err != nil {
					return err
				}
				result = append(result, ExtractedImage{Path: filepath.Join(destination, name), Page: number, Bytes: int64(len(encoded))})
				return nil
			})
		})
		if err != nil {
			return nil, fmt.Errorf("extract images from page %d: %w", number, err)
		}
	}
	if len(result) == 0 {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return nil, err
	}
	for i, item := range result {
		if err := os.Link(filepath.Join(work, filepath.Base(item.Path)), item.Path); err != nil {
			for _, written := range result[:i] {
				_ = os.Remove(written.Path)
			}
			_ = os.Remove(destination)
			return nil, err
		}
	}
	return result, nil
}

func extractImage(doc document, object references.FPDF_PAGEOBJECT) ([]byte, string, error) {
	dimensions, err := doc.FPDFImageObj_GetImagePixelSize(&requests.FPDFImageObj_GetImagePixelSize{ImageObject: object})
	if err != nil {
		return nil, "", err
	}
	if dimensions.Width == 0 || dimensions.Height == 0 || dimensions.Width > (32<<20)/dimensions.Height {
		return nil, "", fmt.Errorf("embedded image exceeds pixel limit or has invalid dimensions")
	}
	filters, err := doc.FPDFImageObj_GetImageFilterCount(&requests.FPDFImageObj_GetImageFilterCount{ImageObject: object})
	if err == nil && filters.Count == 1 {
		filter, err := doc.FPDFImageObj_GetImageFilter(&requests.FPDFImageObj_GetImageFilter{ImageObject: object})
		if err == nil && filter.ImageFilter == "DCTDecode" {
			raw, err := doc.FPDFImageObj_GetImageDataRaw(&requests.FPDFImageObj_GetImageDataRaw{ImageObject: object})
			if err == nil && bytes.HasPrefix(raw.Data, []byte{0xff, 0xd8, 0xff}) {
				return raw.Data, "jpg", nil
			}
		}
	}
	bitmap, err := doc.FPDFImageObj_GetBitmap(&requests.FPDFImageObj_GetBitmap{ImageObject: object})
	if err != nil {
		return nil, "", err
	}
	defer doc.FPDFBitmap_Destroy(&requests.FPDFBitmap_Destroy{Bitmap: bitmap.Bitmap})
	pixels, err := bitmapImage(doc, bitmap.Bitmap)
	if err != nil {
		return nil, "", err
	}
	var output bytes.Buffer
	if err := png.Encode(&output, pixels); err != nil {
		return nil, "", err
	}
	return output.Bytes(), "png", nil
}

func bitmapImage(doc document, ref references.FPDF_BITMAP) (image.Image, error) {
	width, err := doc.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{Bitmap: ref})
	if err != nil {
		return nil, err
	}
	height, err := doc.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{Bitmap: ref})
	if err != nil {
		return nil, err
	}
	stride, err := doc.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{Bitmap: ref})
	if err != nil {
		return nil, err
	}
	format, err := doc.FPDFBitmap_GetFormat(&requests.FPDFBitmap_GetFormat{Bitmap: ref})
	if err != nil {
		return nil, err
	}
	bpp := 4
	switch format.Format {
	case enums.FPDF_BITMAP_FORMAT_GRAY:
		bpp = 1
	case enums.FPDF_BITMAP_FORMAT_BGR:
		bpp = 3
	case enums.FPDF_BITMAP_FORMAT_BGRX, enums.FPDF_BITMAP_FORMAT_BGRA:
	default:
		return nil, fmt.Errorf("unsupported PDFium bitmap format %d", format.Format)
	}
	w, h := width.Width, height.Height
	if w < 1 || h < 1 || w > (32<<20)/h || stride.Stride < w*bpp || stride.Stride > (128<<20)/h {
		return nil, fmt.Errorf("invalid or oversized PDFium bitmap")
	}
	buffer, err := doc.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{Bitmap: ref})
	if err != nil {
		return nil, err
	}
	if len(buffer.Buffer) < stride.Stride*h {
		return nil, fmt.Errorf("incomplete PDFium bitmap")
	}
	output := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := buffer.Buffer[y*stride.Stride:]
		dest := output.Pix[y*output.Stride:]
		for x := 0; x < w; x++ {
			in := row[x*bpp:]
			out := dest[x*4:]
			out[3] = 255
			if bpp == 1 {
				out[0], out[1], out[2] = in[0], in[0], in[0]
				continue
			}
			out[0], out[1], out[2] = in[2], in[1], in[0]
			if format.Format == enums.FPDF_BITMAP_FORMAT_BGRA {
				out[3] = in[3]
			}
		}
	}
	return output, nil
}
