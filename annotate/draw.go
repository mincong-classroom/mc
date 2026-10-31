package annotate

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"regexp"
	"strconv"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Color is the color of a box: green when it validates something, red when it is a flag.
type Color string

const (
	Green Color = "green"
	Red   Color = "red"
)

var (
	markColors = map[Color]color.RGBA{
		Green: {R: 0x2f, G: 0x9e, B: 0x44, A: 0xff},
		Red:   {R: 0xe0, G: 0x31, B: 0x31, A: 0xff},
	}
	white      = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	widenColor = color.RGBA{R: 0xf1, G: 0xf3, B: 0xf5, A: 0xff} // The space added on the right.
	bandColor  = color.RGBA{R: 0x49, G: 0x50, B: 0x57, A: 0xff}
)

// Mark is a box to draw around what the evaluator found, with its color and its label: the number
// of the box, which the text next to the image refers to.
type Mark struct {
	Box
	Color Color
	Label string
}

var markPattern = regexp.MustCompile(`^(\d{1,6}),(\d{1,6}),(\d{1,6}),(\d{1,6}),(green|red),(\w{1,3})$`)

// ParseMark parses a mark written x,y,w,h,color,label, such as 120,40,300,22,green,1: the box as
// Box.String writes it, green or red, and a label of 1 to 3 letters or digits.
func ParseMark(s string) (Mark, error) {
	m := markPattern.FindStringSubmatch(s)
	if m == nil {
		return Mark{}, fmt.Errorf("bad box %q: want x,y,w,h,green|red,label, such as 120,40,300,22,green,1", s)
	}
	var v [4]int
	for i := range v {
		// The pattern allows 6 digits at most, which always fit in an int.
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return Mark{Box: Box{X: v[0], Y: v[1], W: v[2], H: v[3]}, Color: Color(m[5]), Label: m[6]}, nil
}

// Options sets how Draw draws the marks. The zero value draws the marks only.
type Options struct {
	// Padding is the space between a text and its box, in pixels. 0 takes the width of the border:
	// enough to free the text, without crossing the next line of a terminal.
	Padding int
	// Band is a text written in a gray band added at the bottom of the image, such as who took the
	// screenshot and who drew the boxes. No band when it is empty.
	Band string
}

// Draw returns a copy of the image with the marks, drawn like the highlights of slides: a thick
// border around each box, with a padding and no fill, and the label in a circle of its color, on
// the right side of the box. The image is widened when a box ends too close to its right edge.
func Draw(img image.Image, marks []Mark, opts Options) (*image.RGBA, error) {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	stroke := max(3, round(float64(max(width, height))/350))
	padding := opts.Padding
	if padding == 0 {
		padding = stroke
	}
	radius := max(11, round(3.5*float64(stroke)))
	right := width
	for _, m := range marks {
		if _, ok := markColors[m.Color]; !ok {
			return nil, fmt.Errorf("bad color %q of the box %s: green or red", m.Color, m.Label)
		}
		right = max(right, m.X+m.W+padding+2*stroke+2*radius+4)
	}
	out := image.NewRGBA(image.Rect(0, 0, right, height))
	draw.Draw(out, out.Bounds(), image.NewUniform(widenColor), image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(0, 0, width, height), img, b.Min, draw.Src)
	for _, m := range marks {
		c := markColors[m.Color]
		x1, y1 := max(0, m.X-padding), max(0, m.Y-padding)
		x2, y2 := m.X+m.W+padding, min(height-1, m.Y+m.H+padding)
		border(out, image.Rect(x1, y1, x2, y2), stroke, c)
		cx, cy := x2+stroke+radius+2, (y1+y2)/2
		ring := float64(max(1, stroke/2))
		disc(out, cx, cy, float64(radius)+ring/2, white)
		disc(out, cx, cy, float64(radius)-ring/2, c)
		size := float64(radius)
		if len(m.Label) == 1 {
			size *= 1.25
		}
		if err := centered(out, m.Label, size, cx, cy, white); err != nil {
			return nil, err
		}
	}
	if opts.Band == "" {
		return out, nil
	}
	return addBand(out, opts.Band)
}

func round(x float64) int {
	return int(math.RoundToEven(x))
}

// border draws a border of the given width centered on the edges of the rectangle.
func border(dst *image.RGBA, r image.Rectangle, width int, c color.RGBA) {
	in, out := width/2, width-width/2
	outer := image.Rect(r.Min.X-in, r.Min.Y-in, r.Max.X+out, r.Max.Y+out)
	inner := image.Rect(r.Min.X+out, r.Min.Y+out, r.Max.X-in, r.Max.Y-in)
	src := image.NewUniform(c)
	for _, side := range []image.Rectangle{
		image.Rect(outer.Min.X, outer.Min.Y, outer.Max.X, inner.Min.Y), // Top.
		image.Rect(outer.Min.X, inner.Max.Y, outer.Max.X, outer.Max.Y), // Bottom.
		image.Rect(outer.Min.X, inner.Min.Y, inner.Min.X, inner.Max.Y), // Left.
		image.Rect(inner.Max.X, inner.Min.Y, outer.Max.X, inner.Max.Y), // Right.
	} {
		draw.Draw(dst, side, src, image.Point{}, draw.Src)
	}
}

// disc draws a disc centered on the pixel (cx, cy), with antialiased edges.
func disc(dst *image.RGBA, cx, cy int, r float64, c color.RGBA) {
	reach := int(math.Ceil(r)) + 1
	area := image.Rect(cx-reach, cy-reach, cx+reach+1, cy+reach+1)
	mask := image.NewAlpha(area)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			// The share of the pixel inside the disc, roughly: 1 inside, 0 outside, and a ramp of
			// one pixel across the edge.
			coverage := r + 0.5 - math.Hypot(float64(x-cx), float64(y-cy))
			mask.SetAlpha(x, y, color.Alpha{A: uint8(255 * min(1, max(0, coverage)))})
		}
	}
	draw.DrawMask(dst, area, image.NewUniform(c), image.Point{}, mask, area.Min, draw.Over)
}

var boldFont = sync.OnceValues(func() (*opentype.Font, error) { return opentype.Parse(gobold.TTF) })

func face(size float64) (font.Face, error) {
	f, err := boldFont()
	if err != nil {
		return nil, fmt.Errorf("parse the font Go Bold: %w", err)
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// centered writes the text in bold, its ink centered horizontally on cx, and its capital letters
// centered vertically on cy.
func centered(dst draw.Image, text string, size float64, cx, cy int, c color.Color) error {
	f, err := face(size)
	if err != nil {
		return err
	}
	defer f.Close()
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f}
	ink, _ := d.BoundString(text)
	half := fixed.I(1) / 2 // The center of the pixel (cx, cy), as for a disc.
	d.Dot = fixed.Point26_6{
		X: fixed.I(cx) + half - (ink.Min.X+ink.Max.X)/2,
		Y: fixed.I(cy) + half + f.Metrics().CapHeight/2,
	}
	d.DrawString(text)
	return nil
}

// addBand returns a copy of the image with a gray band at the bottom, and the text in it.
func addBand(img *image.RGBA, text string) (*image.RGBA, error) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	band := max(28, round(float64(height)/30))
	out := image.NewRGBA(image.Rect(0, 0, width, height+band))
	draw.Draw(out, img.Bounds(), img, image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(0, height, width, height+band), image.NewUniform(bandColor), image.Point{}, draw.Src)
	size := float64(round(float64(band) * 0.5))
	// A narrow image gets a smaller text, with a margin of half the band on each side.
	f, err := face(size)
	if err != nil {
		return nil, err
	}
	advance := font.MeasureString(f, text).Ceil()
	if err := f.Close(); err != nil {
		return nil, err
	}
	if room := width - band; advance > room {
		if room <= 0 {
			return nil, errors.New("the image is too narrow for its band")
		}
		size *= float64(room) / float64(advance)
	}
	if err := centered(out, text, size, width/2, height+band/2, white); err != nil {
		return nil, err
	}
	return out, nil
}
