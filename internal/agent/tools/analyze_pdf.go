package tools

import (
	"context"
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
)

const analyzePDFFastDescription = "Inspect a local PDF for document-level security indicators and report statistics for the first 25 pages. No rendering, OCR, or text extraction."
const analyzePDFFullDescription = "Inspect a local PDF for document-level security indicators and report statistics for every page. No rendering, OCR, or text extraction."

func execAnalyzePDF(ctx context.Context, env *Env, raw json.RawMessage, pageCount int) (any, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path, err := activatePDFPath(env, args.Path)
	if err != nil {
		return nil, err
	}
	return pdfinspect.Analyze(ctx, path, pageCount)
}
