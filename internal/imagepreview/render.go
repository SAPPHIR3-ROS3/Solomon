package imagepreview

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"strings"
)

type Colors string

const (
	None      Colors = "none"
	ANSI256   Colors = "256"
	TrueColor Colors = "truecolor"
)

type Options struct {
	Width, Height int // maximum terminal columns/rows
	Colors        Colors
	CellAspect    float64 // cell width / height; normally about 0.5
	Background    color.RGBA
}

func (o Options) normalized() (Options, error) {
	if o.Width == 0 {
		o.Width = 80
	}
	if o.Height == 0 {
		o.Height = 40
	}
	if o.Width < 1 || o.Width > 240 || o.Height < 1 || o.Height > 160 {
		return o, fmt.Errorf("preview size must be 1..240 columns and 1..160 rows")
	}

	if o.Colors == "" {
		o.Colors = None
	}
	if o.Colors != None && o.Colors != ANSI256 && o.Colors != TrueColor {
		return o, fmt.Errorf("unknown preview colors %q", o.Colors)
	}
	if o.CellAspect == 0 {
		o.CellAspect = 0.5
	}
	if math.IsNaN(o.CellAspect) || math.IsInf(o.CellAspect, 0) || o.CellAspect < 0.1 || o.CellAspect > 2 {
		return o, fmt.Errorf("cell aspect must be 0.1..2")
	}
	return o, nil
}

type rgb struct{ r, g, b float64 }

func (a rgb) add(b rgb) rgb     { return rgb{a.r + b.r, a.g + b.g, a.b + b.b} }
func (a rgb) mul(s float64) rgb { return rgb{a.r * s, a.g * s, a.b * s} }
func (a rgb) luma() float64     { return .2126*a.r + .7152*a.g + .0722*a.b }
func distance(a, b rgb) float64 {
	r, g, bv := a.r-b.r, a.g-b.g, a.b-b.b
	return .2126*r*r + .7152*g*g + .0722*bv*bv
}
func clamp(x float64) float64 { return math.Max(0, math.Min(255, x)) }
func byteColor(x float64) int { return int(math.Round(clamp(x))) }

// Render emits each completed terminal row immediately. Working samples live
// in a circular window with two rows of history/look-ahead, never a full raster.
// Decode is separate: progressive rendering does not imply streaming decoding.
func Render(ctx context.Context, img image.Image, opts Options, emit func(string) error) error {
	o, err := opts.normalized()
	if err != nil {
		return err
	}
	if img == nil || img.Bounds().Empty() {
		return fmt.Errorf("empty preview image")
	}
	b := img.Bounds()
	w := o.Width
	h := max(1, int(math.Round(float64(w)*float64(b.Dy())/float64(b.Dx())*o.CellAspect)))
	if h > o.Height {
		h = o.Height
		w = max(1, min(o.Width, int(math.Round(float64(h)*float64(b.Dx())/float64(b.Dy())/o.CellAspect))))
	}
	const sx, sy = 8, 8
	win := newWindow(img, w*sx, h*sy, sy+4, o.Background)
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		win.loadThrough(min(h*sy-1, (y+1)*sy+1))
		var line strings.Builder
		lastStyle := ""
		for x := 0; x < w; x++ {
			var samples [64]rgb
			for py := 0; py < sy; py++ {
				for px := 0; px < sx; px++ {
					xx, yy := x*sx+px, y*sy+py
					p := win.at(xx, yy)

					samples[py*sx+px] = p
				}
			}
			ch, fg, bg := fractionCell(samples[:], o.Colors)
			if o.Colors != None {
				style := colorStyle(fg, bg, o.Colors)
				if style != lastStyle {
					line.WriteString(style)
					lastStyle = style
				}
			}
			line.WriteRune(ch)
		}
		if o.Colors != None {
			line.WriteString("\x1b[0m")
		}
		if err := emit(line.String()); err != nil {
			return err
		}
	}
	return nil
}

// Write flushes complete rows, allowing a natural top-to-bottom reveal without
// sleeps, cursor movement, or replacing text already printed in scrollback.
func Write(ctx context.Context, w io.Writer, img image.Image, o Options) error {
	return Render(ctx, img, o, func(row string) error {
		if _, err := io.WriteString(w, row+"\n"); err != nil {
			return err
		}
		if f, ok := w.(interface{ Flush() error }); ok {
			return f.Flush()
		}
		return nil
	})
}

type window struct {
	bounds        image.Rectangle
	img           image.Image
	width, height int
	rows          [][]rgb
	loaded        int
	bg            rgb
}

func newWindow(img image.Image, w, h, n int, bg color.RGBA) *window {
	rows := make([][]rgb, n)
	for i := range rows {
		rows[i] = make([]rgb, w)
	}
	bounds := img.Bounds()
	if c, ok := img.(canvasImage); ok {
		img = c.Image
	}
	return &window{bounds: bounds, img: img, width: w, height: h, rows: rows, loaded: -1, bg: rgb{float64(bg.R), float64(bg.G), float64(bg.B)}}
}
func (w *window) loadThrough(y int) {
	for w.loaded < y {
		w.loaded++
		row := w.rows[w.loaded%len(w.rows)]
		for x := range row {
			row[x] = w.sample(x, w.loaded)
		}
	}
}
func (w *window) at(x, y int) rgb {
	x = max(0, min(w.width-1, x))
	y = max(0, min(w.height-1, y))
	return w.rows[y%len(w.rows)][x]
}

// Exact area averaging when reducing; bilinear interpolation when enlarging.
// Composite alpha before filtering so transparent RGB cannot produce fringes.
func (w *window) sample(x, y int) rgb {
	b := w.bounds
	dx, dy := float64(b.Dx())/float64(w.width), float64(b.Dy())/float64(w.height)
	x0, x1 := float64(x)*dx, float64(x+1)*dx
	y0, y1 := float64(y)*dy, float64(y+1)*dy
	if dx < 1 {
		x0 = (float64(x)+.5)*dx - .5
		x1 = x0 + 1
	}
	if dy < 1 {
		y0 = (float64(y)+.5)*dy - .5
		y1 = y0 + 1
	}
	var sum rgb
	weight := 0.0
	for yy := int(math.Floor(y0)); yy < int(math.Ceil(y1)); yy++ {
		wy := math.Min(y1, float64(yy+1)) - math.Max(y0, float64(yy))
		for xx := int(math.Floor(x0)); xx < int(math.Ceil(x1)); xx++ {
			wx := math.Min(x1, float64(xx+1)) - math.Max(x0, float64(xx))
			a := wx * wy
			ix, iy := b.Min.X+max(0, min(b.Dx()-1, xx)), b.Min.Y+max(0, min(b.Dy()-1, yy))
			p := w.pixel(ix, iy)
			sum = sum.add(p.mul(a))
			weight += a
		}
	}
	return sum.mul(1 / weight)
}

// Read common decoder buffers directly. image.Image.At can allocate a boxed
// color per source pixel, which otherwise dominates both memory and render time.
func (w *window) pixel(x, y int) rgb {
	if !image.Pt(x, y).In(w.img.Bounds()) {
		return w.bg
	}
	var r, g, b, a uint32
	switch m := w.img.(type) {
	case *image.NRGBA:
		i := m.PixOffset(x, y)
		p := m.Pix[i : i+4]
		alpha := float64(p[3]) / 255
		return rgb{float64(p[0])*alpha + w.bg.r*(1-alpha), float64(p[1])*alpha + w.bg.g*(1-alpha), float64(p[2])*alpha + w.bg.b*(1-alpha)}
	case *image.RGBA:
		i := m.PixOffset(x, y)
		p := m.Pix[i : i+4]
		trans := 1 - float64(p[3])/255
		return rgb{float64(p[0]) + w.bg.r*trans, float64(p[1]) + w.bg.g*trans, float64(p[2]) + w.bg.b*trans}
	case *image.Gray:
		v := float64(m.Pix[m.PixOffset(x, y)])
		return rgb{v, v, v}
	case *image.YCbCr:
		yy, cc := m.YOffset(x, y), m.COffset(x, y)
		r, g, b := color.YCbCrToRGB(m.Y[yy], m.Cb[cc], m.Cr[cc])
		return rgb{float64(r), float64(g), float64(b)}
	default:
		r, g, b, a = m.At(x, y).RGBA()
	}
	trans := 1 - float64(a)/65535
	return rgb{float64(r)/257 + w.bg.r*trans, float64(g)/257 + w.bg.g*trans, float64(b)/257 + w.bg.b*trans}
}
