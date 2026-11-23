package sdk

import "encoding/json"

func AnalyzePDFFast(path, intent string) (json.RawMessage, error) {
	return callTool("analyzePDFFast", map[string]string{"path": path, "intent": intent})
}

func AnalyzePDFFull(path, intent string) (json.RawMessage, error) {
	return callTool("analyzePDFFull", map[string]string{"path": path, "intent": intent})
}

func ReadPDFPage(path string, page int, includeImage bool, intent string) (json.RawMessage, error) {
	return callTool("readPDFPage", map[string]any{
		"path": path, "page": page, "includeImage": includeImage, "intent": intent,
	})
}

func ReadPDFPages(path string, firstPage, lastPage int, includeImage bool, intent string) (json.RawMessage, error) {
	return callTool("readPDFPages", map[string]any{
		"path": path, "firstPage": firstPage, "lastPage": lastPage, "includeImage": includeImage, "intent": intent,
	})
}

func SearchInPDF(path, query string, limit int, fuzzy bool, intent string) (json.RawMessage, error) {
	return callTool("searchInPDF", map[string]any{
		"path": path, "query": query, "limit": limit, "fuzzy": fuzzy, "intent": intent,
	})
}

type PDFPageSource struct {
	Path  string `json:"path"`
	Pages []int  `json:"pages"`
}

type PDFEditResult struct {
	Path      string `json:"path"`
	PageCount int    `json:"page_count"`
}

func EditPDFPages(outputPath string, sources []PDFPageSource, intent string) (PDFEditResult, error) {
	return editPDF("editPDFPages", map[string]any{"outputPath": outputPath, "sources": sources, "intent": intent})
}

func MergePDFs(outputPath string, paths []string, intent string) (PDFEditResult, error) {
	return editPDF("mergePDFs", map[string]any{"outputPath": outputPath, "paths": paths, "intent": intent})
}

func DeletePDFPages(path, outputPath string, pages []int, intent string) (PDFEditResult, error) {
	return editPDF("deletePDFPages", map[string]any{"path": path, "outputPath": outputPath, "pages": pages, "intent": intent})
}

func editPDF(name string, args map[string]any) (PDFEditResult, error) {
	var result PDFEditResult
	data, err := callTool(name, args)
	if err == nil {
		err = json.Unmarshal(data, &result)
	}
	return result, err
}

type ExtractedPDFImage struct {
	Path  string `json:"path"`
	Page  int    `json:"page"`
	Bytes int64  `json:"size_bytes"`
}

type PDFImageResult struct {
	OutputDir string              `json:"output_dir"`
	Count     int                 `json:"count"`
	Images    []ExtractedPDFImage `json:"images"`
}

func ExtractPDFImages(path, outputDir string, firstPage, lastPage int, intent string) (PDFImageResult, error) {
	var result PDFImageResult
	raw, err := callTool("extractPDFImages", map[string]any{
		"path": path, "outputDir": outputDir, "firstPage": firstPage, "lastPage": lastPage, "intent": intent,
	})
	if err == nil {
		err = json.Unmarshal(raw, &result)
	}
	return result, err
}

func AnnotatePDF(path, outputPath string, page int, kind string, rect []float32, text, intent string) (PDFEditResult, error) {
	return editPDF("annotatePDF", map[string]any{
		"path": path, "outputPath": outputPath, "page": page, "kind": kind, "rect": rect, "text": text, "intent": intent,
	})
}
