package evidence

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// heading is a heading of the report, outside of the code blocks.
type heading struct {
	line  int // From 1.
	level int
	text  string
}

var (
	headingPattern  = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	fenceOpening    = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	exerciseLikeHdg = regexp.MustCompile(`(?i)^(exercise\s+)?\d+(\.\d+)*([\s:.)-]|$)`)
)

// fence tracks the code blocks of a Markdown file, line by line.
type fence struct {
	marker string // The opening marker of the current code block, empty outside of one.
}

// next reads a line: it reports whether the line is in a code block or is one of its markers.
func (f *fence) next(line string) bool {
	if f.marker != "" {
		trimmed := strings.TrimSpace(line)
		if strings.Trim(trimmed, f.marker[:1]) == "" && len(trimmed) >= len(f.marker) {
			f.marker = ""
		}
		return true
	}
	if m := fenceOpening.FindStringSubmatch(line); m != nil {
		f.marker = m[1]
		return true
	}
	return false
}

func headings(lines []string) []heading {
	var result []heading
	var f fence
	for i, line := range lines {
		if f.next(line) {
			continue
		}
		if m := headingPattern.FindStringSubmatch(line); m != nil {
			result = append(result, heading{line: i + 1, level: len(m[1]), text: m[2]})
		}
	}
	return result
}

// section is the part of the report about an exercise, its heading included.
type section struct {
	heading heading
	start   int // The line of the heading, from 1.
	end     int // The last line, included.
}

// findSection returns the section of the exercise: "4" is the heading "Exercise 4", "4.1" the
// heading "4.1" or "Exercise 4.1". It ends at the next heading of a higher level, or of the same
// level for an exercise, and at any heading of level 2 for a section of level 2.
func findSection(lines []string, exercise string) (section, bool) {
	hs := headings(lines)
	pattern := exercisePattern(exercise)
	for i, h := range hs {
		if h.level < 2 || !pattern.MatchString(h.text) {
			continue
		}
		s := section{heading: h, start: h.line, end: len(lines)}
		for _, next := range hs[i+1:] {
			if next.level < h.level || (next.level == h.level && (h.level == 2 || exerciseLikeHdg.MatchString(next.text))) {
				s.end = next.line - 1
				break
			}
		}
		return s, true
	}
	return section{}, false
}

func exercisePattern(exercise string) *regexp.Regexp {
	prefix := `(exercise\s+)?`
	if !strings.Contains(exercise, ".") {
		prefix = `exercise\s+`
	}
	// The number is not followed by a digit, nor by a part: "Exercise 1" is not "Exercise 10",
	// and "4" is not "4.1".
	return regexp.MustCompile(`(?i)^` + prefix + regexp.QuoteMeta(exercise) + `(\.?([^\d.]|$)|\.?$)`)
}

// block is a paragraph, a heading or a code block of a section.
type block struct {
	start int // The line of its first line, from 1.
	lines []string
	code  bool
	team  int // The number of its lines that the team wrote.
}

// blocks splits the lines of a section, its heading excluded, into blocks. A line is the team's
// when it is not empty, not the marker of a code block, and not a line of the statement. Without
// the statement, every line is the team's.
func blocks(lines []string, first int, statement map[string]bool) []block {
	var result []block
	var current *block
	flush := func() {
		if current != nil && len(current.lines) > 0 {
			result = append(result, *current)
		}
		current = nil
	}
	var f fence
	for i, line := range lines {
		inCode := f.marker != ""
		f.next(line)
		opens := !inCode && f.marker != ""
		closes := inCode && f.marker == ""
		heading := !inCode && !opens && headingPattern.MatchString(line)
		switch {
		case opens:
			flush()
			current = &block{start: first + i, code: true}
		case inCode:
			// A line of the code block, or its closing marker.
		case strings.TrimSpace(line) == "":
			flush()
			continue
		case heading:
			flush()
			current = &block{start: first + i}
		case current == nil:
			current = &block{start: first + i}
		}
		current.lines = append(current.lines, line)
		if trimmed := strings.TrimSpace(line); trimmed != "" && !opens && !closes && !statement[trimmed] {
			current.team++
		}
		if closes || heading {
			flush()
		}
	}
	if f.marker != "" && current != nil {
		// A code block left open would swallow the rest of the page.
		current.lines = append(current.lines, f.marker)
	}
	flush()
	return result
}

var (
	imagePattern    = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)
	htmlImgPattern  = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	linkPattern     = regexp.MustCompile(`(^|[^!])\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)
	externalPattern = regexp.MustCompile(`(?i)^([a-z][a-z0-9+.-]*:|//|#)`)
)

// targets returns the files of the repository that the blocks show as images, and those they link
// to, relative to the root of the repository. The report is at reportPath.
func targets(bs []block, reportPath string) (images, links []string) {
	seen := map[string]bool{}
	add := func(list *[]string, target string) {
		p, ok := repoPath(reportPath, target)
		if ok && !seen[p] {
			seen[p] = true
			*list = append(*list, p)
		}
	}
	for _, b := range bs {
		if b.code {
			continue
		}
		text := strings.Join(b.lines, "\n")
		for _, m := range imagePattern.FindAllStringSubmatch(text, -1) {
			add(&images, m[1])
		}
		for _, m := range htmlImgPattern.FindAllStringSubmatch(text, -1) {
			add(&images, m[1])
		}
		for _, m := range linkPattern.FindAllStringSubmatch(text, -1) {
			add(&links, m[2])
		}
	}
	return images, links
}

// repoPath resolves a link of the report to a path of the repository, or false for a link to a
// web page, an anchor, or outside of the repository.
func repoPath(reportPath, target string) (string, bool) {
	if externalPattern.MatchString(target) {
		return "", false
	}
	if i := strings.IndexAny(target, "#?"); i >= 0 {
		target = target[:i]
	}
	if unescaped, err := url.PathUnescape(target); err == nil {
		target = unescaped
	}
	if target == "" {
		return "", false
	}
	var p string
	if strings.HasPrefix(target, "/") {
		p = path.Clean(strings.TrimPrefix(target, "/")) // GitHub resolves it from the root.
	} else {
		p = path.Join(path.Dir(reportPath), target)
	}
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}
