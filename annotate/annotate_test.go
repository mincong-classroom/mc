package annotate

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The output of tesseract for an image twice as large as the screenshot, with a made-up build: a
// first row in 2 lines, far apart, and a word split in two on the second row.
const tsv = `level	page_num	block_num	par_num	line_num	word_num	left	top	width	height	conf	text
1	1	0	0	0	0	0	0	800	400	-1
4	1	1	1	1	0	20	40	440	30	-1
5	1	1	1	1	1	20	40	200	30	96	BUILD
5	1	1	1	1	2	240	42	220	28	95	SUCCESS
5	1	2	1	1	1	700	44	80	26	88	12:01
5	1	3	1	1	1	20	100	300	30	91	spring–petclinic-4.0.0-SNAPSHOT.
5	1	3	1	1	2	330	100	60	30	90	jar
5	1	3	1	1	3	400	100	20	30	10
`

func TestLines(t *testing.T) {
	lines := groupLines(parseTSV(tsv, 2))
	var got []string
	for _, l := range lines {
		got = append(got, l.Box().String()+" "+l.Text())
	}
	want := []string{
		"10,20,220,15 BUILD SUCCESS",
		"350,22,40,13 12:01",
		"10,50,185,15 spring–petclinic-4.0.0-SNAPSHOT. jar",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestFind(t *testing.T) {
	lines := groupLines(parseTSV(tsv, 2))
	for _, c := range []struct {
		text string
		line int
		want string // The box, or "" when the line doesn't contain the text.
	}{
		{text: "success", line: 0, want: "120,21,110,14"},
		{text: "Build Success", line: 0, want: "10,20,220,15"},
		{text: "SNAPSHOT.jar", line: 2, want: "10,50,185,15"},
		{text: "spring-petclinic", line: 2, want: "10,50,150,15"}, // An en dash in the screenshot.
		{text: "failure", line: 0, want: ""},
		{text: " ", line: 0, want: ""},
	} {
		b, ok := lines[c.line].Find(c.text)
		got := ""
		if ok {
			got = b.String()
		}
		if got != c.want {
			t.Errorf("lines[%d].Find(%q) = %q, want %q", c.line, c.text, got, c.want)
		}
	}
}

func TestParseMark(t *testing.T) {
	m, err := ParseMark("120,40,300,22,green,1")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Mark{Box: Box{X: 120, Y: 40, W: 300, H: 22}, Color: Green, Label: "1"}); m != want {
		t.Errorf("ParseMark() = %+v, want %+v", m, want)
	}
	for _, s := range []string{"120,40,300,green,1", "120,40,300,22,blue,1", "120,40,300,22,red,1234", "-1,40,300,22,red,1"} {
		if _, err := ParseMark(s); err == nil {
			t.Errorf("ParseMark(%q) = nil error, want one", s)
		}
	}
}

func blank(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

func TestDraw(t *testing.T) {
	// A box far from the right edge: the size stays the same, with a border of 3 pixels, a padding
	// of 3, and a circle of a radius of 11 on the right.
	img := blank(400, 200)
	out, err := Draw(img, []Mark{{Box: Box{X: 20, Y: 40, W: 100, H: 20}, Color: Green, Label: "1"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Bounds().Size(); got != image.Pt(400, 200) {
		t.Errorf("size = %v, want (400,200)", got)
	}
	green := markColors[Green]
	for _, c := range []struct {
		name string
		x, y int
		want color.RGBA
	}{
		{name: "the left border", x: 17, y: 50, want: green},
		{name: "the top border", x: 70, y: 37, want: green},
		{name: "inside the box", x: 70, y: 50, want: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		{name: "the left of the circle", x: 123 + 3 + 11 + 2 - 8, y: 50, want: green},
		{name: "the white ring", x: 123 + 3 + 11 + 2, y: 50 - 11, want: white},
	} {
		if got := out.RGBAAt(c.x, c.y); got != c.want {
			t.Errorf("%s, at (%d,%d) = %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
	if got := img.RGBAAt(17, 50); got != (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Errorf("Draw changed its input: %v at (17,50)", got)
	}
}

func TestDrawWidens(t *testing.T) {
	out, err := Draw(blank(400, 200), []Mark{{Box: Box{X: 300, Y: 40, W: 90, H: 20}, Color: Red, Label: "2"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// 300 + 90 + the padding of 3, 2 borders of 3, 2 radii of 11 and 4 pixels.
	if got := out.Bounds().Dx(); got != 425 {
		t.Errorf("width = %d, want 425", got)
	}
	if got := out.RGBAAt(420, 5); got != widenColor {
		t.Errorf("the space added, at (420,5) = %v, want %v", got, widenColor)
	}
	if got := out.RGBAAt(393, 50); got != markColors[Red] {
		t.Errorf("the right border, at (393,50) = %v, want %v", got, markColors[Red])
	}
}

func TestDrawBand(t *testing.T) {
	out, err := Draw(blank(400, 200), nil, Options{Band: "Screenshot of the team."})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Bounds().Size(); got != image.Pt(400, 228) {
		t.Errorf("size = %v, want (400,228): a band of 28 pixels at least", got)
	}
	if got := out.RGBAAt(2, 226); got != bandColor {
		t.Errorf("the band, at (2,226) = %v, want %v", got, bandColor)
	}
	if got := out.RGBAAt(2, 198); got != (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Errorf("above the band, at (2,198) = %v, want white", got)
	}
	// A narrow image keeps the whole text, in a smaller size.
	if _, err := Draw(blank(120, 60), nil, Options{Band: "Screenshot of the team. The numbered boxes were added by the evaluator."}); err != nil {
		t.Errorf("Draw() of a narrow image = %v", err)
	}
}

func TestDrawBadColor(t *testing.T) {
	if _, err := Draw(blank(40, 20), []Mark{{Color: "blue", Label: "1"}}, Options{}); err == nil {
		t.Error("Draw() of a blue box = nil error, want one")
	}
}

// TestRead runs tesseract on a made-up screenshot, when it is installed.
func TestRead(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract is not installed")
	}
	img := blank(600, 120)
	if err := centered(img, "BUILD SUCCESS", 28, 300, 60, color.Black); err != nil {
		t.Fatal(err)
	}
	lines, err := Read(context.Background(), img)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if b, ok := l.Find("build success"); ok {
			// The text is about 230 pixels wide and 20 high, around (300, 60).
			if b.X < 150 || b.X+b.W > 450 || b.Y < 35 || b.Y+b.H > 85 {
				t.Errorf("the box of the text = %v, want around (300,60)", b)
			}
			return
		}
	}
	t.Errorf("Read() = %v, want a line BUILD SUCCESS", lines)
}

func TestSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "boxed.png")
	if err := Save(p, blank(30, 10)); err != nil {
		t.Fatal(err)
	}
	img, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != image.Pt(30, 10) {
		t.Errorf("Load() size = %v, want (30,10)", got)
	}
}
