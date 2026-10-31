package evidence

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/rubric"
)

// maxImportLines is the number of lines of a file, or of its changes, imported into the report. A
// longer one is linked: the evaluator writes an excerpt of its key lines.
const maxImportLines = 80

// report is the evidence of a lab for a team.
type report struct {
	lab          rubric.Lab
	deadline     time.Time
	team         string
	commit       commitInfo
	reportFound  bool
	teamLines    int // The lines that the team wrote in its report.
	statement    bool
	rules        []ruleEvidence
	otherFiles   []fileEvidence // The files of the team that no rule names, and the report doesn't link.
	otherByEval  []evaluatorFile
	evidencePath string // The report of the evidence, e.g. docs/lab-1.evidence.md.
	evidenceDir  string // The directory of the evaluator, e.g. docs/lab-1.evidence.
	revision     string // The commit of mc, "unknown" when not given.
}

// ruleEvidence is the evidence of a rule.
type ruleEvidence struct {
	rule     rubric.Rule
	section  *sectionEvidence // Nil when the report has no section for the exercise.
	files    []fileEvidence
	notes    string // The notes of the evaluator, in {ID}.md of its directory.
	byEval   []evaluatorFile
	images   []imageEvidence
	notesRel string // The path of the notes, relative to the report.
}

type sectionEvidence struct {
	heading       string
	start, end    int
	teamLines     int
	blocks        []block // The blocks with a line of the team.
	missingImages []string
}

// fileEvidence is a file of the repository of the team.
type fileEvidence struct {
	path    string
	why     string // Why it is there: "a file of the rule", "linked from the report".
	status  string // Such as "new, 12 lines" or "changed since the template".
	missing bool
	image   bool
	content string // What is imported: the file, or its changes since the template.
	lang    string // The language of the code block of the content.
	hygiene bool   // A file that doesn't belong in a repository, such as a .DS_Store.
	tooLong bool   // A text too long to import.
}

// evaluatorFile is a file of the directory of the evaluator.
type evaluatorFile struct {
	name    string // Relative to the directory of the evaluator.
	image   bool
	lines   int
	content string
	lang    string
}

// imageEvidence is a Docker image of a rule, on Docker Hub.
type imageEvidence struct {
	reference  string // Such as namespace/app-red:1.0.0.
	repository string
	tag        string
	found      bool // Whether the repository exists.
	tags       []tagEvidence
	err        string
}

type tagEvidence struct {
	tag   Tag
	patch bool // A later patch of the tag of the rule, which counts for it.
	late  bool // Last pushed after the deadline.
}

type buildInput struct {
	rubric   *rubric.Rubric
	lab      rubric.Lab
	deadline time.Time
	team     Team
	commit   commitInfo
	clone    git
	worktree string
	template *Template
	registry Registry
	revision string
}

func build(in buildInput) (*report, error) {
	r := &report{lab: in.lab, deadline: in.deadline, team: in.team.Name, commit: in.commit, revision: in.revision}
	if r.revision == "" {
		r.revision = "unknown"
	}
	r.evidencePath, r.evidenceDir = evidencePaths(in.lab.Report)
	tree, err := in.clone.tree(in.commit.hash)
	if err != nil {
		return nil, err
	}
	files := fileReader{clone: in.clone, tree: tree, template: in.template}

	var lines []string
	var statement map[string]bool
	if e, ok := tree[in.lab.Report]; ok {
		content, err := in.clone.blob(e.blob)
		if err != nil {
			return nil, err
		}
		lines = strings.Split(string(content), "\n")
		r.reportFound = true
		if statement, err = in.template.statementLines(in.lab.Report); err != nil {
			return nil, err
		}
		r.statement = statement != nil
		for _, b := range blocks(lines, 1, statement) {
			r.teamLines += b.team
		}
	}

	byEval, err := evaluatorFiles(in.worktree, r.evidenceDir)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	attributed := map[string]bool{in.lab.Report: true}
	tags := map[string][]Tag{}
	for _, rule := range in.rubric.LabRules(in.lab.ID) {
		re := ruleEvidence{rule: rule, notesRel: path.Join(path.Base(r.evidenceDir), rule.ID+".md")}
		var links []string
		if sec, ok := findSection(lines, rule.Exercise); ok {
			se := &sectionEvidence{heading: sec.heading.text, start: sec.start, end: sec.end}
			for _, b := range blocks(lines[sec.start:sec.end], sec.start+1, statement) {
				se.teamLines += b.team
				if b.team > 0 || statement == nil {
					se.blocks = append(se.blocks, b)
				}
			}
			images, linked := targets(se.blocks, in.lab.Report)
			for _, image := range images {
				attributed[image] = true
				if _, ok := tree[image]; !ok {
					se.missingImages = append(se.missingImages, image)
				}
			}
			links = linked
			re.section = se
		}
		for _, p := range rule.Files.Team {
			attributed[p] = true
			f, err := files.read(p, "a file of the rule")
			if err != nil {
				return nil, err
			}
			re.files = append(re.files, f)
		}
		for _, p := range links {
			if slicesContainsPath(re.files, p) {
				continue
			}
			attributed[p] = true
			f, err := files.read(p, "linked from the report")
			if err != nil {
				return nil, err
			}
			re.files = append(re.files, f)
		}
		for _, f := range byEval {
			switch {
			case f.name == rule.ID+".md":
				re.notes = f.content
				used[f.name] = true
			case ruleFilePattern(rule.ID).MatchString(f.name):
				re.byEval = append(re.byEval, f)
				used[f.name] = true
			}
		}
		for _, image := range rule.Images {
			re.images = append(re.images, lookUpImage(in.registry, tags, strings.ReplaceAll(image, "{team}", in.team.Name), in.deadline))
		}
		r.rules = append(r.rules, re)
	}

	for _, f := range byEval {
		if !used[f.name] {
			r.otherByEval = append(r.otherByEval, f)
		}
	}
	var others []string
	for p, e := range tree {
		if attributed[p] || in.template.fromTemplate(p, e.blob) || ofAnotherLab(p, in.lab.Number()) {
			continue
		}
		others = append(others, p)
	}
	sort.Strings(others)
	for _, p := range others {
		f, err := files.read(p, "")
		if err != nil {
			return nil, err
		}
		r.otherFiles = append(r.otherFiles, f)
	}
	return r, nil
}

func slicesContainsPath(files []fileEvidence, p string) bool {
	for _, f := range files {
		if f.path == p {
			return true
		}
	}
	return false
}

// ruleFilePattern matches the files of the evaluator for a rule: l1-dkf-build.log or
// L1_DKF.png for L1_DKF.
func ruleFilePattern(id string) *regexp.Regexp {
	parts := strings.SplitN(strings.ToLower(id), "_", 2)
	return regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(parts[0]) + `[_-]` + regexp.QuoteMeta(parts[len(parts)-1]) + `([._-]|$)`)
}

var otherLabPattern = regexp.MustCompile(`^(k8s/lab-(\d+)/|docs/lab-(\d+)[./])`)

// ofAnotherLab reports whether the path belongs to another lab, such as k8s/lab-2/ for Lab 1.
func ofAnotherLab(p, number string) bool {
	m := otherLabPattern.FindStringSubmatch(p)
	if m == nil {
		return false
	}
	n := m[2]
	if n == "" {
		n = m[3]
	}
	return n != number
}

// fileReader reads the files of the team at the commit evaluated.
type fileReader struct {
	clone    git
	tree     map[string]entry
	template *Template
}

var hygienePattern = regexp.MustCompile(`(?i)(^|/)(\.DS_Store|Thumbs\.db|desktop\.ini)$|(^|/)(target|node_modules|\.idea|__pycache__)/|\.(class|jar|war|iml|exe|dll|so|dylib|pyc)$`)

func (fr fileReader) read(p, why string) (fileEvidence, error) {
	f := fileEvidence{path: p, why: why, hygiene: hygienePattern.MatchString(p)}
	e, ok := fr.tree[p]
	if !ok {
		f.missing = true
		f.status = "missing at the commit evaluated"
		return f, nil
	}
	if isImage(p) {
		f.image = true
		f.status = fmt.Sprintf("an image of %s", size(e.size))
		if fr.template.fromTemplate(p, e.blob) {
			f.status += ", the one of the template"
		}
		return f, nil
	}
	if e.size == 0 {
		f.status = "empty"
		f.hygiene = true
		return f, nil
	}
	content, err := fr.clone.blob(e.blob)
	if err != nil {
		return f, err
	}
	if bytes.IndexByte(content[:min(len(content), 8000)], 0) >= 0 {
		f.status = fmt.Sprintf("binary: %s", size(e.size))
		f.hygiene = true
		return f, nil
	}
	lines := countLines(content)
	if fr.template.fromTemplate(p, e.blob) {
		f.status = fmt.Sprintf("unchanged since the template: %s", plural(lines, "line"))
		return f, nil
	}
	diff, changed, found, err := fr.template.diffWithTemplate(p, content)
	if err != nil {
		return f, err
	}
	if found {
		f.status = fmt.Sprintf("changed since the template: %s added or removed, of %s", plural(changed, "line"), plural(lines, "line"))
		if countLines([]byte(diff)) <= maxImportLines {
			f.content, f.lang = diff, "diff"
		} else {
			f.tooLong = true
		}
		return f, nil
	}
	f.status = fmt.Sprintf("new: %s", plural(lines, "line"))
	if lines <= maxImportLines {
		f.content, f.lang = string(content), language(p)
	} else {
		f.tooLong = true
	}
	return f, nil
}

// evaluatorFiles returns the files of the directory of the evaluator, in the worktree.
func evaluatorFiles(worktree, dir string) ([]evaluatorFile, error) {
	root := filepath.Join(worktree, filepath.FromSlash(dir))
	var files []evaluatorFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return fs.SkipAll
		}
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		f := evaluatorFile{name: filepath.ToSlash(rel), image: isImage(p)}
		if !f.image {
			content, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			f.lines = countLines(content)
			if f.lines <= maxImportLines || strings.HasSuffix(p, ".md") {
				f.content, f.lang = string(content), language(p)
			}
		}
		files = append(files, f)
		return nil
	})
	return files, err
}

var semverPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// lookUpImage reads the tag of the image on Docker Hub, and its later patches, such as 1.1.1 for
// 1.1.0, which count for it.
func lookUpImage(registry Registry, cache map[string][]Tag, reference string, deadline time.Time) imageEvidence {
	i := strings.LastIndex(reference, ":")
	ie := imageEvidence{reference: reference, repository: reference[:i], tag: reference[i+1:]}
	if registry == nil {
		ie.err = "Docker Hub was not read"
		return ie
	}
	tags, ok := cache[ie.repository]
	if !ok {
		var found bool
		var err error
		tags, found, err = registry.Tags(ie.repository)
		if err != nil {
			ie.err = err.Error()
			return ie
		}
		if !found {
			return ie
		}
		cache[ie.repository] = tags
	}
	ie.found = true
	wanted := semverPattern.FindStringSubmatch(ie.tag)
	for _, tag := range tags {
		te := tagEvidence{tag: tag, late: tag.LastPushed.After(deadline)}
		switch {
		case tag.Name == ie.tag:
		case wanted != nil && isLaterPatch(tag.Name, wanted):
			te.patch = true
		default:
			continue
		}
		ie.tags = append(ie.tags, te)
	}
	sort.SliceStable(ie.tags, func(a, b int) bool { return ie.tags[a].tag.Name < ie.tags[b].tag.Name })
	return ie
}

func isLaterPatch(name string, wanted []string) bool {
	m := semverPattern.FindStringSubmatch(name)
	if m == nil || m[1] != wanted[1] || m[2] != wanted[2] {
		return false
	}
	patch, _ := strconv.Atoi(m[3])        // The pattern matches digits only.
	minimum, _ := strconv.Atoi(wanted[3]) // The pattern matches digits only.
	return patch > minimum
}

func isImage(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return true
	}
	return false
}

func countLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	n := bytes.Count(content, []byte("\n"))
	if content[len(content)-1] != '\n' {
		n++
	}
	return n
}

func language(p string) string {
	name := strings.ToLower(path.Base(p))
	if name == "dockerfile" || strings.HasSuffix(name, ".dockerfile") || strings.HasPrefix(name, "dockerfile.") {
		return "dockerfile"
	}
	switch path.Ext(name) {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".xml":
		return "xml"
	case ".java":
		return "java"
	case ".properties":
		return "properties"
	case ".sh":
		return "bash"
	case ".ps1":
		return "powershell"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	}
	return "text"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	if strings.HasSuffix(word, "sh") || strings.HasSuffix(word, "s") {
		return strconv.Itoa(n) + " " + word + "es" // 2 pushes, not 2 pushs.
	}
	return strconv.Itoa(n) + " " + word + "s"
}

func size(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%d KB", bytes>>10)
	}
	return plural(int(bytes), "byte")
}
