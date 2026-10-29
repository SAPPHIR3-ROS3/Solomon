package imagepreview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func renderRows(t *testing.T, img image.Image, o Options) []string {
	t.Helper()
	var rows []string
	if err := Render(context.Background(), img, o, func(s string) error { rows = append(rows, s); return nil }); err != nil {
		t.Fatal(err)
	}
	return rows
}
func solid(w, h int, c color.Color) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Set(x, y, c)
		}
	}
	return m
}
func TestDimensionsAndColors(t *testing.T) {
	for _, colors := range []Colors{None, ANSI256, TrueColor} {
		rows := renderRows(t, solid(120, 60, color.White), Options{Width: 40, Height: 40, Colors: colors})
		if len(rows) != 10 {
			t.Fatalf("%s rows: %d", colors, len(rows))
		}
		for _, r := range rows {
			if utf8.RuneCountInString(regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(r, "")) != 40 {
				t.Fatal("width changed")
			}
		}
		tall := renderRows(t, solid(10, 100, color.White), Options{Width: 40, Height: 20, Colors: colors})
		if len(tall) != 20 || utf8.RuneCountInString(regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(tall[0], "")) != 4 {
			t.Fatal("height bound/aspect lost")
		}
	}
}
func TestColorAndAlpha(t *testing.T) {
	m := solid(2, 4, color.NRGBA{R: 255, A: 0})
	for _, c := range []Colors{TrueColor, ANSI256, None} {
		rows := renderRows(t, m, Options{Width: 1, Height: 1, Colors: c, Background: color.RGBA{B: 255, A: 255}})
		switch c {
		case TrueColor:
			if !strings.Contains(rows[0], "38;2;0;0;255;48;2;0;0;255m") {
				t.Fatalf("transparent RGB leaked: %q", rows)
			}
		case ANSI256:
			if !strings.Contains(rows[0], "38;5;21;48;5;21m") {
				t.Fatalf("wrong palette blue: %q", rows)
			}
		case None:
			if strings.Contains(rows[0], "\x1b") {
				t.Fatal("ANSI in plain output")
			}
		}
	}
}
func TestWindowKeepsRowsAcrossWrap(t *testing.T) {
	// Alternating four-pixel bands reveal lost history/look-ahead and ring wrap.
	m := solid(2, 80, color.Black)
	for y := 0; y < 80; y++ {
		if (y/4)%2 == 0 {
			for x := 0; x < 2; x++ {
				m.Set(x, y, color.White)
			}
		}
	}
	rows := renderRows(t, m, Options{Width: 1, Height: 20})
	if len(rows) != 20 {
		t.Fatal(len(rows))
	}
	for y, r := range rows {
		want := " "
		if y%2 == 0 {
			want = "█"
		}
		if r != want {
			t.Fatalf("row %d got %q", y, r)
		}
	}
}
func TestCancellationAndWriterFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	count := 0
	err := Render(ctx, solid(20, 20, color.White), Options{Width: 20}, func(string) error { count++; cancel(); return nil })
	if !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatalf("cancel: %d %v", count, err)
	}
	want := errors.New("writer failed")
	if err := Write(context.Background(), brokenWriter{want}, solid(2, 2, color.White), Options{}); !errors.Is(err, want) {
		t.Fatal(err)
	}
	for _, o := range []Options{{Width: -1}, {Width: 241}, {Height: 161}, {Colors: "bad"}, {CellAspect: -1}} {
		if err := Render(context.Background(), solid(1, 1, color.Black), o, func(string) error { return nil }); err == nil {
			t.Fatalf("accepted invalid option: %+v", o)
		}
	}
}

type brokenWriter struct{ err error }

func (w brokenWriter) Write([]byte) (int, error) { return 0, w.err }
func TestDecodeFirstGIFFrameAndOffset(t *testing.T) {
	palette := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	first := image.NewPaletted(image.Rect(1, 1, 3, 3), palette)
	for i := range first.Pix {
		first.Pix[i] = 1
	}
	second := image.NewPaletted(image.Rect(0, 0, 4, 4), palette)
	for i := range second.Pix {
		second.Pix[i] = 2
	}
	var encoded bytes.Buffer
	err := gif.EncodeAll(&encoded, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{1, 1}, Config: image.Config{ColorModel: palette, Width: 4, Height: 4}})
	if err != nil {
		t.Fatal(err)
	}
	img, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 4, 4) {
		t.Fatal("lost logical canvas")
	}
	r, _, b, _ := img.At(1, 1).RGBA()
	if r != 65535 || b != 0 {
		t.Fatal("decoded wrong frame")
	}
	// Pixels outside the offset first frame must be composited onto the backdrop.
	rows := renderRows(t, img, Options{Width: 8, Height: 4, Colors: TrueColor, Background: color.RGBA{G: 255, A: 255}})
	if !strings.Contains(rows[0], "0;255;0") {
		t.Fatal("lost GIF canvas background")
	}
}
func TestDecodeLimitsAndBrokenInput(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not an image"), make([]byte, MaxBytes+1)} {
		if _, err := Decode(bytes.NewReader(data)); err == nil {
			t.Fatal("accepted invalid/oversized input")
		}
	}
	// A huge image header is rejected before allocating decoded pixels.
	var b bytes.Buffer
	if err := png.Encode(&b, solid(1, 1, color.White)); err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), b.Bytes()...)
	data[16] = 0x7f
	data[17] = 0xff
	data[18] = 0xff
	data[19] = 0xff
	if _, err := Decode(bytes.NewReader(data)); err == nil {
		t.Fatal("accepted broken dimensions")
	}
}
func TestColorDetection(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	if DetectColors(true) != TrueColor || DetectColors(false) != None {
		t.Fatal("wrong truecolor/pipe detection")
	}
	t.Setenv("COLORTERM", "")
	if DetectColors(true) != ANSI256 {
		t.Fatal("no 256 fallback")
	}
	t.Setenv("NO_COLOR", "1")
	if DetectColors(true) != None {
		t.Fatal("NO_COLOR ignored")
	}
}
func BenchmarkRender(b *testing.B) {
	img := solid(1280, 720, color.RGBA{R: 80, G: 160, B: 240, A: 255})
	for _, colors := range []Colors{None, ANSI256, TrueColor} {
		b.Run(string(colors), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := Write(context.Background(), io.Discard, img, Options{Width: 80, Height: 40, Colors: colors}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestPaletteChoosesNearestStableColor(t *testing.T) {
	levels := [6]float64{0, 95, 135, 175, 215, 255}
	for r := 0; r <= 255; r += 51 {
		for g := 0; g <= 255; g += 51 {
			for b := 0; b <= 255; b += 51 {
				input := rgb{float64(r), float64(g), float64(b)}
				index := paletteIndex(input)
				decode := func(n int) rgb {
					if n >= 232 {
						v := float64(8 + (n-232)*10)
						return rgb{v, v, v}
					}
					n -= 16
					return rgb{levels[n/36], levels[(n/6)%6], levels[n%6]}
				}
				chosen := distance(input, decode(index))
				for i := 16; i < 256; i++ {
					if distance(input, decode(i)) < chosen-1e-6 {
						t.Fatalf("%v: index %d is worse than %d", input, index, i)
					}
				}
			}
		}
	}
}

func TestOptimizedPixelsAgreeWithImageInterface(t *testing.T) {
	// Verify common fast paths against the generic color interface, including
	// premultiplied alpha and a source image whose bounds do not begin at zero.
	rect := image.Rect(3, 5, 7, 9)
	rgba := image.NewRGBA(rect)
	nrgba := image.NewNRGBA(rect)
	gray := image.NewGray(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := color.NRGBA{uint8(x * 25), uint8(y * 20), 75, 128}
			rgba.Set(x, y, c)
			nrgba.Set(x, y, c)
			gray.Set(x, y, c)
		}
	}
	bg := color.RGBA{R: 20, G: 40, B: 60, A: 255}
	for _, img := range []image.Image{rgba, nrgba, gray} {
		fast := newWindow(img, 4, 4, 8, bg)
		slow := newWindow(genericImage{img}, 4, 4, 8, bg)
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				if d := distance(fast.pixel(x, y), slow.pixel(x, y)); d > 1 {
					t.Fatalf("pixel drift for %T: %f", img, d)
				}
			}
		}
	}
}

type genericImage struct{ image.Image }
