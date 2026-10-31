// Package annotate finds a text in a screenshot, then draws numbered boxes around what the
// evaluator found: green when a box validates something, red when it is a flag.
package annotate

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// ocrScale is how much an image is enlarged before tesseract reads it: tesseract reads the small
// text of a screenshot better when it is larger.
const ocrScale = 2

// mitchell is the filter Mitchell-Netravali, with B = C = 1/3: the one of ImageMagick to enlarge an
// image, whose output tesseract was first tuned on.
var mitchell = &xdraw.Kernel{Support: 2, At: func(t float64) float64 {
	const b, c = 1.0 / 3, 1.0 / 3
	switch t = math.Abs(t); {
	case t < 1:
		return ((12-9*b-6*c)*t*t*t + (-18+12*b+6*c)*t*t + (6 - 2*b)) / 6
	case t < 2:
		return ((-b-6*c)*t*t*t + (6*b+30*c)*t*t + (-12*b-48*c)*t + (8*b + 24*c)) / 6
	default:
		return 0
	}
}}

// Box is a rectangle of an image, in pixels: its top left corner, its width and its height.
type Box struct {
	X, Y, W, H int
}

// String returns the box as x,y,w,h, the way a Mark starts.
func (b Box) String() string {
	return fmt.Sprintf("%d,%d,%d,%d", b.X, b.Y, b.W, b.H)
}

// Word is a word that tesseract reads in an image, with its box.
type Word struct {
	Box
	Text string
}

// Line is a line of text that tesseract reads in an image: its words, from left to right.
type Line struct {
	Words []Word
}

// Box returns the box around the words of the line.
func (l Line) Box() Box {
	return boxOf(l.Words)
}

// Text returns the words of the line, separated by spaces.
func (l Line) Text() string {
	texts := make([]string, len(l.Words))
	for i, w := range l.Words {
		texts[i] = w.Text
	}
	return strings.Join(texts, " ")
}

// Find returns the box around the words of the line that contain the text. It ignores the case,
// the spaces and the kind of dash: tesseract often splits a word in two, e.g. "SNAPSHOT. jar", and
// reads a long dash as a hyphen.
func (l Line) Find(text string) (Box, bool) {
	needle := normalize(text)
	if needle == "" {
		return Box{}, false
	}
	var joined strings.Builder
	for _, w := range l.Words {
		joined.WriteString(normalize(w.Text))
	}
	start := strings.Index(joined.String(), needle)
	if start < 0 {
		return Box{}, false
	}
	var covered []Word
	offset := 0
	for _, w := range l.Words {
		end := offset + len(normalize(w.Text))
		if end > start && offset < start+len(needle) {
			covered = append(covered, w)
		}
		offset = end
	}
	return boxOf(covered), true
}

var dashes = strings.NewReplacer("—", "-", "–", "-")

// normalize lowers the case, removes the spaces, and reads a long dash as a hyphen.
func normalize(s string) string {
	return dashes.Replace(strings.Join(strings.Fields(strings.ToLower(s)), ""))
}

func boxOf(words []Word) Box {
	x1, y1 := math.MaxInt, math.MaxInt
	x2, y2 := math.MinInt, math.MinInt
	for _, w := range words {
		x1, y1 = min(x1, w.X), min(y1, w.Y)
		x2, y2 = max(x2, w.X+w.W), max(y2, w.Y+w.H)
	}
	return Box{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// Read returns the lines of text that tesseract reads in the image, from top to bottom. It needs
// the command tesseract.
func Read(ctx context.Context, img image.Image) ([]Line, error) {
	b := img.Bounds()
	scaled := image.NewRGBA(image.Rect(0, 0, b.Dx()*ocrScale, b.Dy()*ocrScale))
	mitchell.Scale(scaled, scaled.Bounds(), img, b, xdraw.Src, nil)
	var in bytes.Buffer
	if err := png.Encode(&in, scaled); err != nil {
		return nil, fmt.Errorf("encode the image for tesseract: %w", err)
	}
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "--psm", "11", "tsv")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = &in, &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tesseract: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return groupLines(parseTSV(out.String(), ocrScale)), nil
}

// parseTSV returns the words of the output TSV of tesseract, in the coordinates of an image scale
// times smaller than the one that tesseract read.
func parseTSV(tsv string, scale int) []Word {
	var words []Word
	for i, row := range strings.Split(tsv, "\n") {
		f := strings.Split(strings.TrimRight(row, "\r"), "\t")
		// The header comes first, and the level 5 is a word: the levels 1 to 4 are the page, the
		// blocks, the paragraphs and the lines, which --psm 11 splits too much to be useful.
		if i == 0 || len(f) < 12 || f[0] != "5" || strings.TrimSpace(f[11]) == "" {
			continue
		}
		var v [4]int
		valid := true
		for j := range v {
			n, err := strconv.Atoi(f[6+j])
			if err != nil {
				valid = false
				break
			}
			v[j] = n / scale
		}
		if valid {
			words = append(words, Word{Box: Box{X: v[0], Y: v[1], W: v[2], H: v[3]}, Text: f[11]})
		}
	}
	return words
}

// groupLines groups the words in lines. With --psm 11 (sparse text), tesseract splits a line in
// pieces, which may overlap: group the words by row first, then split a row where a large gap
// separates two of its words.
func groupLines(words []Word) []Line {
	center := func(w Word) float64 { return float64(w.Y) + float64(w.H)/2 }
	sorted := slices.Clone(words)
	slices.SortStableFunc(sorted, func(a, b Word) int { return cmp.Compare(center(a), center(b)) })
	type row struct {
		center, height float64 // Those of the first word of the row.
		words          []Word
	}
	var rows []row
	for _, w := range sorted {
		c := center(w)
		if n := len(rows); n > 0 && math.Abs(c-rows[n-1].center) < max(4, 0.5*rows[n-1].height) {
			rows[n-1].words = append(rows[n-1].words, w)
			continue
		}
		rows = append(rows, row{center: c, height: float64(w.H), words: []Word{w}})
	}
	var lines []Line
	for _, r := range rows {
		slices.SortStableFunc(r.words, func(a, b Word) int { return cmp.Compare(a.X, b.X) })
		line := []Word{r.words[0]}
		for _, w := range r.words[1:] {
			last := line[len(line)-1]
			if w.X-(last.X+last.W) > max(30, 3*last.H) {
				lines = append(lines, Line{Words: line})
				line = []Word{w}
				continue
			}
			line = append(line, w)
		}
		lines = append(lines, Line{Words: line})
	}
	slices.SortStableFunc(lines, func(a, b Line) int {
		return cmp.Or(cmp.Compare(a.Words[0].Y, b.Words[0].Y), cmp.Compare(a.Words[0].X, b.Words[0].X))
	})
	return lines
}
