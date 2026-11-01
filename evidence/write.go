package evidence

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/rubric"
)

const timeLayout = "Mon 2 Jan 2006, 15:04 (UTC-07:00)"

// markdown returns the Markdown of the facts. Its links are relative to its directory, the one
// of the report of the team.
func (r *labFacts) markdown() string {
	var b strings.Builder
	dir := path.Dir(r.factsPath)
	link := func(p string) string { return relativeLink(dir, p) }
	evalDir := path.Base(r.evidenceDir)
	at := func(t time.Time) string { return t.In(r.deadline.Location()).Format(timeLayout) }

	b.WriteString(metadata{Commit: r.commit.hash, MC: r.revision}.frontMatter())
	fmt.Fprintf(&b, "# Lab %s: %s, Team %s\n\n", r.lab.Number(), r.lab.Title, r.team)
	fmt.Fprintf(&b, "The facts of each rule, for the teacher: what the team wrote in its report, the files of its repository, and the evidence that the evaluator made. Facts, not a score. `mc eval gather-facts` generates this file: the evaluator writes in [`%s/`](%s/), and `mc eval gather-facts` runs again.\n\n", evalDir, evalDir)

	c := r.commit
	commit := fmt.Sprintf("- Commit evaluated: `%s`, %s", short(c.hash), c.how)
	if !c.late && !c.none {
		commit += fmt.Sprintf(" (%s)", at(r.deadline))
	}
	if !c.pushedAt.IsZero() {
		commit += fmt.Sprintf(", pushed on %s", at(c.pushedAt))
	}
	b.WriteString(commit + "\n")
	fmt.Fprintf(&b, "- Gathered by `mc` at `%s`\n", r.revision)
	if len(c.later) > 0 {
		last := c.later[len(c.later)-1]
		verb := "not in this evaluation"
		if c.late {
			verb = "in this evaluation, a late pass"
		}
		fmt.Fprintf(&b, "- Pushed after the deadline: %s, the last one on %s, up to `%s`: %s\n", plural(len(c.later), "push"), at(last.Time), short(last.After), verb)
	} else if c.activity {
		b.WriteString("- Pushed after the deadline: nothing\n")
	}
	if len(c.forced) > 0 {
		var times []string
		for _, push := range c.forced {
			times = append(times, at(push.Time))
		}
		fmt.Fprintf(&b, "- History rewritten: %s, on %s\n", plural(len(c.forced), "force push"), strings.Join(times, "; "))
	}
	if r.reportFound {
		report := fmt.Sprintf("- Report of the team: [`%s`](%s)", r.lab.Report, link(r.lab.Report))
		if r.statement {
			report += fmt.Sprintf(", %s of the team", plural(r.teamLines, "line"))
		}
		b.WriteString(report + "\n")
	} else {
		fmt.Fprintf(&b, "- Report of the team: `%s` is missing at the commit evaluated\n", r.lab.Report)
	}

	b.WriteString("\n| Rule | Exercise | Name | In the report | In the repository | By the evaluator |\n|---|---|---|---|---|---|\n")
	for _, re := range r.rules {
		inReport := "no section"
		if re.section != nil {
			inReport = plural(re.section.teamLines, "line")
		}
		fmt.Fprintf(&b, "| [`%s`](#%s) | %s | %s | %s | %s | %s |\n", re.rule.ID, anchor(ruleHeading(re.rule)), re.rule.Exercise, re.rule.Name, inReport, plural(len(re.files), "file"), byEvaluator(re))
	}

	for _, re := range r.rules {
		fmt.Fprintf(&b, "\n## %s\n\n", ruleHeading(re.rule))
		if asked := evidenceList(re.rule.Evidence.Asked); asked != "" {
			fmt.Fprintf(&b, "Asked: %s.", asked)
			if extra := evidenceList(re.rule.Evidence.Extra); extra != "" {
				fmt.Fprintf(&b, " Observed, not in the score: %s.", extra)
			}
			b.WriteString("\n")
		}

		b.WriteString("\n### In the Report\n\n")
		switch s := re.section; {
		case s == nil && r.reportFound:
			fmt.Fprintf(&b, "No section for Exercise %s in the report.\n", re.rule.Exercise)
		case s == nil:
			b.WriteString("No report.\n")
		default:
			fmt.Fprintf(&b, "[%s](%s#L%d), lines %d to %d", s.heading, link(r.lab.Report), s.start, s.start, s.end)
			switch {
			case r.statement && s.teamLines == 0:
				b.WriteString(": no line of the team, only the statement")
			case r.statement:
				fmt.Fprintf(&b, ": %s of the team", plural(s.teamLines, "line"))
			}
			b.WriteString(".\n")
			for _, block := range s.blocks {
				b.WriteString("\n" + strings.Join(block.lines, "\n") + "\n")
			}
			for _, image := range s.missingImages {
				fmt.Fprintf(&b, "\nThe image `%s` is missing at the commit evaluated.\n", image)
			}
		}

		b.WriteString("\n### In the Repository\n")
		if len(re.files) == 0 {
			b.WriteString("\nNo file of the rule, and the report links to none.\n")
		}
		for _, f := range re.files {
			writeFile(&b, f, link)
		}

		b.WriteString("\n### By the Evaluator\n")
		if re.notes == "" && len(re.byEval) == 0 && len(re.images) == 0 {
			fmt.Fprintf(&b, "\nNothing yet: the evaluator writes its notes in [`%s`](%s), and saves its screenshots and logs as `%s/%s-*`.\n", re.notesRel, re.notesRel, evalDir, strings.ReplaceAll(strings.ToLower(re.rule.ID), "_", "-"))
		}
		if re.notes != "" {
			b.WriteString("\n" + strings.TrimSpace(re.notes) + "\n")
		}
		if len(re.images) > 0 {
			writeImages(&b, re.images, at)
		}
		for _, f := range re.byEval {
			writeEvaluatorFile(&b, f, evalDir)
		}
	}

	if len(r.otherFiles) > 0 {
		b.WriteString("\n## Other Files of the Team\n\nThe files that the team added or changed for this lab, or outside of the labs, which no rule names and the report doesn't link.\n")
		for _, f := range r.otherFiles {
			writeFile(&b, f, link)
		}
	}
	if len(r.otherByEval) > 0 {
		b.WriteString("\n## Other Files of the Evaluator\n")
		for _, f := range r.otherByEval {
			writeEvaluatorFile(&b, f, evalDir)
		}
	}
	return b.String()
}

// byEvaluator sums up the evidence of the evaluator for the rule, such as "notes, 2 files".
func byEvaluator(re ruleEvidence) string {
	var parts []string
	if re.notes != "" {
		parts = append(parts, "notes")
	}
	if len(re.byEval) > 0 {
		parts = append(parts, plural(len(re.byEval), "file"))
	}
	if len(re.images) > 0 {
		parts = append(parts, plural(len(re.images), "image")+" on Docker Hub")
	}
	if len(parts) == 0 {
		return "nothing yet"
	}
	return strings.Join(parts, ", ")
}

func writeFile(b *strings.Builder, f fileEvidence, link func(string) string) {
	fmt.Fprintf(b, "\n#### [`%s`](%s)\n\n", f.path, link(f.path))
	status := f.status
	if f.why != "" {
		status = f.why + ", " + status
	}
	if f.hygiene {
		status += ". Not expected in a repository"
	}
	b.WriteString(capitalize(status) + ".\n")
	switch {
	case f.image && !f.missing:
		fmt.Fprintf(b, "\n![%s](%s)\n", path.Base(f.path), link(f.path))
	case f.content != "":
		writeCode(b, f.content, f.lang)
	case f.tooLong:
		b.WriteString("\nToo long to import: the evaluator quotes its key lines in its notes.\n")
	}
}

func writeEvaluatorFile(b *strings.Builder, f evaluatorFile, dir string) {
	p := path.Join(dir, f.name)
	fmt.Fprintf(b, "\n#### [`%s`](%s)\n", f.name, p)
	switch {
	case f.image:
		fmt.Fprintf(b, "\n![%s](%s)\n", f.name, p)
	case f.content != "" && f.lang == "markdown":
		b.WriteString("\n" + strings.TrimSpace(f.content) + "\n")
	case f.content != "":
		writeCode(b, f.content, f.lang)
	default:
		fmt.Fprintf(b, "\n%s: too long to import.\n", capitalize(plural(f.lines, "line")))
	}
}

func writeImages(b *strings.Builder, images []imageEvidence, at func(time.Time) string) {
	b.WriteString("\nOn Docker Hub, read by `mc eval gather-facts`:\n\n| Image | Last pushed | Platforms | Digest |\n|---|---|---|---|\n")
	for _, ie := range images {
		switch {
		case ie.err != "":
			fmt.Fprintf(b, "| `%s` | not read: %s | | |\n", ie.reference, ie.err)
		case !ie.found:
			fmt.Fprintf(b, "| `%s` | no repository `%s` | | |\n", ie.reference, ie.repository)
		case len(ie.tags) == 0:
			fmt.Fprintf(b, "| `%s` | no tag `%s` | | |\n", ie.reference, ie.tag)
		}
		for _, te := range ie.tags {
			name := fmt.Sprintf("`%s:%s`", ie.repository, te.tag.Name)
			if te.patch {
				name += ", a patch that counts for it"
			}
			pushed := at(te.tag.LastPushed)
			if te.late {
				pushed += ", after the deadline"
			}
			fmt.Fprintf(b, "| %s | %s | %s | `%s` |\n", name, pushed, strings.Join(te.tag.Platforms(), ", "), shortDigest(te.tag.Digest))
		}
	}
}

// writeCode writes the content in a code block whose fence is longer than any in the content.
func writeCode(b *strings.Builder, content, lang string) {
	fence := "```"
	for strings.Contains(content, fence) {
		fence += "`"
	}
	fmt.Fprintf(b, "\n%s%s\n%s\n%s\n", fence, lang, strings.TrimRight(content, "\n"), fence)
}

func ruleHeading(rule rubric.Rule) string {
	heading := fmt.Sprintf("`%s` %s (Ex %s)", rule.ID, rule.Name, rule.Exercise)
	if rule.Optional() {
		heading += ", Bonus"
	}
	return heading
}

func evidenceList(items []rubric.EvidenceItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = item.Kind + ", " + item.What
	}
	return strings.Join(parts, "; ")
}

var anchorDrop = regexp.MustCompile(`[^a-z0-9 _-]`)

// anchor returns the anchor of a heading, as GitHub and VS Code make it.
func anchor(heading string) string {
	return strings.ReplaceAll(anchorDrop.ReplaceAllString(strings.ToLower(heading), ""), " ", "-")
}

// relativeLink returns the path of the repository relative to the directory, for a link.
func relativeLink(dir, p string) string {
	if dir == "." || dir == "" {
		return p
	}
	up := strings.Repeat("../", strings.Count(dir, "/")+1)
	if strings.HasPrefix(p, dir+"/") {
		return strings.TrimPrefix(p, dir+"/")
	}
	return up + p
}

func shortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
