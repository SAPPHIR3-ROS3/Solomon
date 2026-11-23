package tools

import (
	"context"
	"encoding/json"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
)

const extractPDFImagesDescription = "Extract embedded PDF images into a new local directory as PNG or JPEG, with source page numbers. Optional 1-based inclusive firstPage and lastPage; zero leaves that bound open. Refuses to overwrite the destination."

func execExtractPDFImages(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var args struct {
		Path   string `json:"path"`
		Output string `json:"outputDir"`
		First  int    `json:"firstPage"`
		Last   int    `json:"lastPage"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	path, err := activatePDFPath(env, args.Path)
	if err != nil {
		return nil, err
	}
	output, err := activatePDFPath(env, args.Output)
	if err != nil {
		return nil, err
	}
	return WithFileChangeTrackingPaths(ctx, env, []string{output}, func() (any, error) {
		stageBeforeEdit(env, output)
		extracted, err := pdfinspect.ExtractImages(ctx, path, output, args.First, args.Last)
		if err != nil {
			return nil, err
		}
		for _, item := range extracted {
			stageAfterEdit(env, "create", item.Path, "")
			stageProjectPath(env, item.Path)
		}
		return map[string]any{"output_dir": output, "count": len(extracted), "images": extracted}, nil
	})
}
