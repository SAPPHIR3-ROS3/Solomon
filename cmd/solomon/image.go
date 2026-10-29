package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/imagepreview"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/terminalmedia"
)

// This standalone path runs before config, authentication, desktop or daemon
// setup. It is also useful for comparing render modes on local fixtures.
func runImageCLI(ctx context.Context, args []string, out, errOut io.Writer) error {
	defaults := terminalmedia.Options(out)
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	fs.SetOutput(errOut)
	colors := fs.String("color", "auto", "auto, truecolor, 256, or none (explicit values also work in pipes)")
	width := fs.Int("width", defaults.Width, "maximum columns (1..240)")
	height := fs.Int("height", defaults.Height, "maximum rows (1..160)")
	aspect := fs.Float64("cell-aspect", .5, "cell width divided by height")
	background := fs.String("background", "000000", "RGB background for transparency, e.g. ffffff")
	preview := fs.String("preview", "", "poster image for a video or other media")
	fs.Usage = func() { fmt.Fprintln(errOut, "Usage: solomon image [options] <local-file>"); fs.PrintDefaults() }
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one local media path")
	}
	hex := strings.TrimPrefix(*background, "#")
	n, err := strconv.ParseUint(hex, 16, 24)
	if err != nil || len(hex) != 6 {
		return fmt.Errorf("background must contain six RGB hex digits")
	}
	path, err := filepath.Abs(expandPathArg(fs.Arg(0)))
	if err != nil {
		return err
	}
	source := path
	if *preview != "" {
		source = expandPathArg(*preview)
	}
	img, err := imagepreview.Open(source)
	if err != nil {
		return err
	}
	defaults.Width, defaults.Height = *width, *height
	defaults.CellAspect = *aspect
	defaults.Background = color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
	if *colors != "auto" {
		defaults.Colors = imagepreview.Colors(*colors)
	}
	if err := imagepreview.Write(ctx, out, img, defaults); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, path)
	return err
}
