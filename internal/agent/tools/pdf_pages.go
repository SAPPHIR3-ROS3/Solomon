package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
)

const readPDFPageDescription = "Extract native text from one local PDF page (1-based). Set includeImage to also attach the rendered page for vision. No OCR."
const readPDFPagesDescription = "Extract native text from an inclusive, 1-based PDF page range, with a separate result per page. Set includeImage to also attach the rendered pages in order. Limits: 25 pages for text or 5 with images. No OCR."
const searchInPDFDescription = "Search native text across a local PDF using BM25 over blocks of four sentences with one sentence of overlap. Returns ranked passages and page numbers. Set fuzzy to allow a one-edit fallback for unmatched query terms. No OCR."

func execReadPDFPages(ctx context.Context, env *Env, raw json.RawMessage, single bool) (any, error) {
	var args struct {
		Path         string `json:"path"`
		Page         int    `json:"page"`
		First        int    `json:"firstPage"`
		Last         int    `json:"lastPage"`
		IncludeImage bool   `json:"includeImage"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if single {
		args.First, args.Last = args.Page, args.Page
	}
	path, err := activatePDFPath(env, args.Path)
	if err != nil {
		return nil, err
	}
	pages, renders, warned, err := pdfinspect.ReadPages(ctx, path, args.First, args.Last, args.IncludeImage)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"path": path, "pages": pages, "parser_warning": warned}
	if len(renders) > 0 {
		attachments := make([]images.Attachment, 0, len(renders))
		for i, render := range renders {
			attachments = append(attachments, images.Attachment{
				Path:     fmt.Sprintf("%s#page=%d", path, pages[i].Number),
				MIMEType: "image/jpeg", Data: render,
			})
		}
		result[images.ToolAttachmentsKey] = attachments
	}
	return result, nil
}

func execSearchInPDF(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	args := struct {
		Path  string `json:"path"`
		Query string `json:"query"`
		Limit int    `json:"limit"`
		Fuzzy bool   `json:"fuzzy"`
	}{Limit: 8}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path, err := activatePDFPath(env, args.Path)
	if err != nil {
		return nil, err
	}
	return pdfinspect.Search(ctx, path, args.Query, args.Limit, args.Fuzzy)
}

func activatePDFPath(env *Env, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("PDF path is required")
	}
	path = resolveProjectPath(env.ProjRoot, path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(path)
	}
	return path, nil
}
