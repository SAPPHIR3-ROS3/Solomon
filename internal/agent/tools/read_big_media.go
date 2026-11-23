package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/imagepreview"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images"
	"golang.org/x/image/draw"
)

const readBigMediaDescription = "Read a local PNG, JPEG, WebP, or GIF image and attach a resized PNG copy within the 5 MiB vision attachment limit. GIFs use the first frame. Input limits are 32 MiB and 32 MiPixels. Accepts project-relative or absolute paths."

func execReadBigMedia(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, fmt.Errorf("readBigMedia: image path is required")
	}
	path := resolveProjectPath(env.ProjRoot, args.Path)
	if env.ActivateInstructionsFromAbsPath != nil {
		env.ActivateInstructionsFromAbsPath(path)
	}
	source, err := imagepreview.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readBigMedia: %w", err)
	}
	original := source.Bounds()
	width, height := original.Dx(), original.Dy()
	longest := max(width, height)
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	var encoded bytes.Buffer
	for edge := min(longest, 2048); edge > 0; edge = edge * 3 / 4 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		bounds := image.Rect(0, 0, max(1, int(int64(width)*int64(edge)/int64(longest))), max(1, int(int64(height)*int64(edge)/int64(longest))))
		preview := image.NewNRGBA(bounds)
		draw.ApproxBiLinear.Scale(preview, bounds, source, original, draw.Src, nil)
		encoded.Reset()
		if err := encoder.Encode(&encoded, preview); err != nil {
			return nil, fmt.Errorf("readBigMedia: encode preview: %w", err)
		}
		if encoded.Len() <= maxReadImageBytes {
			attachment := images.Attachment{Path: path, MIMEType: "image/png", Data: encoded.Bytes()}
			result := readImageResult(path, "image/png", bounds.Dx(), bounds.Dy(), encoded.Len(), attachment)
			result["original_width"] = width
			result["original_height"] = height
			return result, nil
		}
	}
	return nil, fmt.Errorf("readBigMedia: unable to fit preview within 5 MiB")
}
