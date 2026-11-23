package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/pdfinspect"
)

const editPDFPagesDescription = "Create a new PDF from an ordered list of source PDFs and selected 1-based pages. Omit pages to include an entire source. May omit document-level features such as bookmarks. Refuses to overwrite."
const mergePDFsDescription = "Merge local PDFs in the supplied order into a new PDF. Refuses to overwrite."
const deletePDFPagesDescription = "Create a new PDF omitting specified 1-based pages, preserving the order of the remaining pages. Refuses to overwrite."

func execEditPDF(ctx context.Context, env *Env, raw json.RawMessage, tool string) (any, error) {
	var args struct {
		Output  string                  `json:"outputPath"`
		Sources []pdfinspect.PageSource `json:"sources"`
		Paths   []string                `json:"paths"`
		Path    string                  `json:"path"`
		Pages   []int                   `json:"pages"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Output) == "" {
		return nil, fmt.Errorf("outputPath is required")
	}
	sources := args.Sources
	var deleted []int
	switch tool {
	case "mergePDFs":
		if len(args.Paths) < 2 {
			return nil, fmt.Errorf("mergePDFs requires at least two paths")
		}
		sources = make([]pdfinspect.PageSource, 0, len(args.Paths))
		for _, path := range args.Paths {
			sources = append(sources, pdfinspect.PageSource{Path: path})
		}
	case "deletePDFPages":
		if len(args.Pages) == 0 {
			return nil, fmt.Errorf("at least one page to delete is required")
		}
		sources = []pdfinspect.PageSource{{Path: args.Path}}
		deleted = args.Pages
	}
	for i := range sources {
		path, err := activatePDFPath(env, sources[i].Path)
		if err != nil {
			return nil, err
		}
		sources[i].Path = path
	}
	output, err := activatePDFPath(env, args.Output)
	if err != nil {
		return nil, err
	}
	return writePDFTool(ctx, env, output, func() (int, error) {
		return pdfinspect.EditPages(ctx, output, sources, deleted)
	})
}

func writePDFTool(ctx context.Context, env *Env, output string, write func() (int, error)) (any, error) {
	return WithFileChangeTrackingPaths(ctx, env, []string{output}, func() (any, error) {
		stageBeforeEdit(env, output)
		count, err := write()
		if err != nil {
			return nil, err
		}
		stageAfterEdit(env, "create", output, "")
		stageProjectPath(env, output)
		return map[string]any{"path": output, "page_count": count}, nil
	})
}

const annotatePDFDescription = "Add a native note or highlight to a new PDF copy. Set a 1-based page and a rectangle [left, bottom, right, top] in unrotated PDF points. Notes require text. Refuses to overwrite."

func execAnnotatePDF(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var args struct {
		Path   string `json:"path"`
		Output string `json:"outputPath"`
		pdfinspect.Annotation
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
	return writePDFTool(ctx, env, output, func() (int, error) {
		return pdfinspect.Annotate(ctx, path, output, args.Annotation)
	})
}
