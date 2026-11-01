package evaluation

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/sheet"
)

// The tab of the evaluation sheet, and the headers of its columns that Sync writes.
const (
	SheetTab = "Evaluation"

	headerTeam         = "Team"
	headerLab          = "Lab"
	headerRule         = "Rule"
	headerRuleScore    = "Rule Score"
	headerCompleteness = "Completeness"
	headerScore        = "Score"
	headerToCheck      = "TA To Check"
	headerCommit       = "TA Commit"
	headerAnalysis     = "TA Analysis"
	headerWritingStyle = "TA Writing Style"
	headerTimeSpent    = "TA Time Spent"
	headerLabFlag      = "TA Lab Flag"
	headerPenalty      = "TA Penalty"
	headerActionItems  = "TA Action Items"
	headerComment      = "TA Comment"
)

// sheetKeys are the headers whose values find a row of the tab.
var sheetKeys = []string{headerTeam, headerLab, headerRule}

// sheetColumns are the columns of the tab that mc reads, by their letters. B and C are not among
// them: they hold the students of the team, which mc never reads, and their formulas are filled
// down from the row above. I and J are the teacher's: mc reads them, and never writes them.
var sheetColumns = map[string]string{
	"A": headerTeam,
	"D": headerLab,
	"E": headerRule,
	"F": headerRuleScore,
	"G": headerCompleteness,
	"H": headerScore,
	"I": "Comment",
	"J": "Troubleshooting Instructions",
	"K": headerToCheck,
	"L": headerCommit,
	"M": "TA Docker Images",
	"N": "TA Screenshots",
	"O": headerAnalysis,
	"P": headerWritingStyle,
	"Q": headerTimeSpent,
	"R": headerLabFlag,
	"S": headerPenalty,
	"T": headerActionItems,
	"U": headerComment,
}

// SheetFile returns an empty file of the tab Evaluation, with the columns that mc reads: the
// layout of the script that reads the tab.
func SheetFile() *sheet.File {
	columns := make(map[string]string, len(sheetColumns))
	for letter, header := range sheetColumns {
		columns[letter] = header
	}
	return &sheet.File{Tab: SheetTab, Keys: slices.Clone(sheetKeys), Columns: columns}
}

// SyncOptions are the options of Sync.
type SyncOptions struct {
	Lab string // The ID of the lab, such as L2.
	// Files are the evaluations of the lab, one per team, checked.
	Files []*File
	// Read is the tab, as read after the last paste.
	Read *sheet.File
	// Exported is what mc wrote last in the tab, nil before the first export: a cell of the tab
	// that differs from it was changed by someone else, such as the teacher.
	Exported *sheet.File
	// Pending is what waited to be written, nil if nothing. Its rows of the lab and the teams of
	// Files are replaced, and the others wait still.
	Pending *sheet.File
}

// SyncResult is what Sync computed.
type SyncResult struct {
	// Pending is what waits to be written now: the file of mc sheet.
	Pending *sheet.File
	// Changes are the cells of the rows of the tab that the sync writes, or keeps, in the order of
	// the rows.
	Changes []Change
	// Added are the new rows of the sync, by team.
	Added []Added
	// Replaced is the number of rows of the lab and the teams that waited, and that the sync
	// replaced.
	Replaced int
	// FillFrom is the last row of the tab, whose B and C are filled down to the last new row that
	// waits, LastNewRow. Both are 0 without a new row.
	FillFrom, LastNewRow int
}

// Change is a cell of a row of the tab that the sync writes, or keeps.
type Change struct {
	Cell   string // Such as Evaluation!G81.
	Row    string // The keys of the row: teal L2 L2_POD.
	Header string
	From   any // The value in the tab, nil when the cell is empty.
	To     any // The value written, or the value of the evaluation when the cell is kept.
	// Check says why the teacher should look at the cell before it is written, if anything.
	Check string
	// Kept says why the cell stays as it is, empty when it is written.
	Kept string
}

// Added are new rows of a team, which follow each other.
type Added struct {
	Team        string
	First, Last int
}

// Sync computes the cells that write the evaluations of a lab in the tab, by the rules of a pass:
//
//   - A row is found by its team, its lab and its rule. A missing row is added after the last
//     row, with its formulas.
//   - The completeness is written in an empty cell, or over the one that mc wrote last when it is
//     higher, with its TA Comment. A lower one keeps the former value, and TA To Check says why.
//     A completeness that mc didn't write, such as one of the teacher, is kept.
//   - The other TA columns follow the evaluation, but a cell is never cleared. A cell that mc
//     didn't write, and that the evaluation changes, waits with a check for the teacher.
//   - A penalty becomes the formula of TA Penalty, and its reason goes to the TA Comment.
func Sync(opts SyncOptions) (*SyncResult, error) {
	if opts.Read == nil {
		return nil, errors.New("no read of the tab")
	}
	if err := checkColumns("the tab read", opts.Read, true); err != nil {
		return nil, err
	}
	s := syncer{
		lab:      opts.Lab,
		read:     rowsByKey(opts.Read),
		readAt:   map[int]sheet.Row{},
		exported: map[string]sheet.Row{},
		reuse:    map[string]int{},
		result:   &SyncResult{Pending: SheetFile()},
	}
	for _, row := range opts.Read.Rows {
		s.readAt[row.Number] = row
		s.last = max(s.last, row.Number)
	}
	s.result.FillFrom = s.last
	if opts.Exported != nil {
		s.exported = rowsByKey(opts.Exported)
	}
	teams := map[string]bool{}
	for _, f := range opts.Files {
		teams[f.Team] = true
	}
	if opts.Pending != nil {
		if err := s.keepWaiting(opts.Pending, teams); err != nil {
			return nil, err
		}
	}

	files := slices.Clone(opts.Files)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Team < files[j].Team })
	for _, f := range files {
		for _, rule := range f.Rules {
			if err := s.sync(f, rule); err != nil {
				return nil, err
			}
		}
	}

	pending := s.result.Pending
	sort.SliceStable(pending.Rows, func(i, j int) bool { return pending.Rows[i].Number < pending.Rows[j].Number })
	if problems := pending.Problems(); len(problems) > 0 {
		return nil, fmt.Errorf("the rows to write are not valid: %s", strings.Join(problems, "; "))
	}
	for _, row := range pending.Rows {
		if row.New {
			s.result.LastNewRow = max(s.result.LastNewRow, row.Number)
		}
	}
	if s.result.LastNewRow == 0 {
		s.result.FillFrom = 0
	}
	return s.result, nil
}

// checkColumns returns an error if the file, the tab read or what waits, doesn't have the layout of
// mc: its tab, its keys, and its columns. all requires each column of the layout.
func checkColumns(what string, f *sheet.File, all bool) error {
	if f.Tab != SheetTab || !slices.Equal(f.Keys, sheetKeys) {
		return fmt.Errorf("%s is the tab %q with the keys %q, not %q with %q", what, f.Tab, f.Keys, SheetTab, sheetKeys)
	}
	letters := make([]string, 0, len(sheetColumns))
	for letter := range sheetColumns {
		letters = append(letters, letter)
	}
	sort.Strings(letters) // One letter each: the order of the columns.
	for _, letter := range letters {
		want := sheetColumns[letter]
		switch got, ok := f.Columns[letter]; {
		case !ok && all:
			return fmt.Errorf("%s has no column %s, %q: read the tab with the script of mc eval sync", what, letter, want)
		case ok && got != want:
			return fmt.Errorf("the column %s of %s is %q, whereas mc writes %q there: the layout of the tab changed", letter, what, got, want)
		}
	}
	for letter, header := range f.Columns {
		if _, ok := sheetColumns[letter]; !ok {
			return fmt.Errorf("%s has the column %s, %q, which mc never reads", what, letter, header)
		}
	}
	return nil
}

// syncer computes the rows of a sync.
type syncer struct {
	lab      string
	read     map[string]sheet.Row // The rows of the tab, by their keys.
	readAt   map[int]sheet.Row    // The rows of the tab, by their numbers.
	exported map[string]sheet.Row // The rows of the last export, by their keys.
	reuse    map[string]int       // The numbers of the new rows of the sync that waited, by their keys.
	last     int                  // The last row of the tab, or of what waits.
	result   *SyncResult
}

// keepWaiting keeps the rows that wait, but those of the lab and the teams, which the sync
// replaces. A row that waits must still find its place in the tab.
func (s *syncer) keepWaiting(pending *sheet.File, teams map[string]bool) error {
	if err := checkColumns("what waits", pending, false); err != nil {
		return err
	}
	for _, row := range pending.Rows {
		k := rowKey(row)
		s.last = max(s.last, row.Number)
		lab := text(row.Cells[headerLab])
		if lab == s.lab && teams[text(row.Cells[headerTeam])] {
			s.result.Replaced++
			if row.New {
				s.reuse[k] = row.Number
			}
			continue
		}
		name := describe(row)
		if row.New {
			if r, ok := s.readAt[row.Number]; ok {
				return fmt.Errorf("the new row %d of %s waits, whereas the tab has the row %d of %s now: sync %s again", row.Number, name, r.Number, describe(r), lab)
			}
		} else if r, ok := s.read[k]; !ok {
			return fmt.Errorf("the row %d of %s waits, whereas the tab no longer has it: sync %s again", row.Number, name, lab)
		} else if r.Number != row.Number {
			return fmt.Errorf("the row %d of %s waits, whereas the tab has it in the row %d now: sync %s again", row.Number, name, r.Number, lab)
		}
		s.result.Pending.Rows = append(s.result.Pending.Rows, row)
	}
	return nil
}

// sync computes the row of a rule of the file, and adds it to what waits if it has a cell to
// write.
func (s *syncer) sync(f *File, rule Rule) error {
	r := rowSync{file: f, rule: rule, result: s.result}
	k := key3(f.Team, s.lab, rule.ID)
	if read, ok := s.read[k]; ok {
		r.row = sheet.Row{Number: read.Number, Cells: map[string]any{}}
		r.read, r.exported = read, s.exported[k]
		if err := r.existing(); err != nil {
			return err
		}
	} else {
		number, ok := s.reuse[k]
		if !ok {
			s.last++
			number = s.last
		}
		r.row = sheet.Row{Number: number, Cells: map[string]any{}, New: true}
		if err := r.added(); err != nil {
			return err
		}
		s.addRow(f.Team, number)
	}
	if len(r.row.Cells) == 0 {
		return nil
	}
	r.row.Cells[headerTeam] = f.Team
	r.row.Cells[headerLab] = s.lab
	r.row.Cells[headerRule] = rule.ID
	s.result.Pending.Rows = append(s.result.Pending.Rows, r.row)
	return nil
}

// addRow records a new row of the team.
func (s *syncer) addRow(team string, number int) {
	added := s.result.Added
	if n := len(added); n > 0 && added[n-1].Team == team && added[n-1].Last+1 == number {
		added[n-1].Last = number
		return
	}
	s.result.Added = append(added, Added{Team: team, First: number, Last: number})
}

// rowSync computes the cells of the row of a rule.
type rowSync struct {
	file     *File
	rule     Rule
	result   *SyncResult
	row      sheet.Row // The cells to write.
	read     sheet.Row // The row of the tab, empty for a new row.
	exported sheet.Row // The row of the last export, empty if none.
}

// added writes a new row: its formulas, then its cells.
func (r *rowSync) added() error {
	n := strconv.Itoa(r.row.Number)
	r.row.Cells[headerRuleScore] = "=VLOOKUP(E" + n + ",Rules!C:F,4,FALSE)"
	r.row.Cells[headerScore] = "=G" + n + "*F" + n + "+S" + n
	sum := "SUMIFS($H:$H,$A:$A,A" + n + ",$D:$D,D" + n + ")"
	r.row.Cells[headerLabFlag] = `=IF(` + sum + `<12,"Below 12/20 ("&ROUND(` + sum + `,1)&")","")`
	r.set(headerCompleteness, completenessCell(r.rule.Completeness))
	if err := r.setText(headerComment, r.comment()); err != nil {
		return err
	}
	if err := r.setText(headerToCheck, r.rule.ToCheck); err != nil {
		return err
	}
	return r.taCells()
}

// existing writes the cells of a row of the tab that change.
func (r *rowSync) existing() error {
	toCheck := r.rule.ToCheck
	former, filled := number(r.read.Cells[headerCompleteness])
	written, wrote := number(r.exported.Cells[headerCompleteness])
	own := filled && wrote && equal(former, written)
	switch {
	case !filled, own && r.rule.Completeness > former+epsilon:
		r.set(headerCompleteness, completenessCell(r.rule.Completeness))
		if err := r.setText(headerComment, r.comment()); err != nil {
			return err
		}
	case own && r.rule.Completeness < former-epsilon:
		r.keep(headerCompleteness, completenessCell(r.rule.Completeness), "lower than the former pass")
		toCheck = joinSentences(fmt.Sprintf("The pass of %s gives %s, lower than %s, which is kept: what changed?",
			short(r.file.Commit), formatFloat(r.rule.Completeness), formatFloat(former)), toCheck)
	case own:
		// The same completeness: its comment stays.
	default:
		r.keep(headerCompleteness, completenessCell(r.rule.Completeness), "not written by mc, such as a value of the teacher")
		toCheck = joinSentences(fmt.Sprintf("The pass of %s gives %s, whereas the tab has %s, which mc didn't write: it is kept.",
			short(r.file.Commit), formatFloat(r.rule.Completeness), formatFloat(former)), toCheck)
	}
	if err := r.setText(headerToCheck, toCheck); err != nil {
		return err
	}
	if r.rule.Penalty == nil && !isEmpty(r.read.Cells[headerPenalty]) {
		r.keep(headerPenalty, nil, "a penalty that this pass doesn't give: clear it by hand if it no longer applies")
	}
	return r.taCells()
}

// taCells writes the TA columns that follow the evaluation, and the penalty.
func (r *rowSync) taCells() error {
	texts := []struct{ header, value string }{
		{headerCommit, short(r.file.Commit)},
		{headerAnalysis, r.rule.Analysis},
		{headerWritingStyle, r.rule.WritingStyle},
		{headerTimeSpent, r.file.TimeSpent},
		{headerActionItems, r.file.ActionItems},
	}
	for _, t := range texts {
		if err := r.setText(t.header, t.value); err != nil {
			return err
		}
	}
	if p := r.rule.Penalty; p != nil {
		n := strconv.Itoa(r.row.Number)
		r.set(headerPenalty, "=-"+formatFloat(p.Remove)+"*G"+n+"*F"+n)
	}
	return nil
}

// setText writes a text on one line, unless it is empty: a cell is never cleared.
func (r *rowSync) setText(header, value string) error {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "=") {
		return fmt.Errorf("%s %s, %s: %q starts with =, which the tab would read as a formula", r.file.Team, r.rule.ID, header, value)
	}
	r.set(header, value)
	return nil
}

// set writes a cell, unless the tab holds its value already. In a row of the tab, a cell that holds
// another value, which mc didn't write, such as one of the teacher, waits with a check.
func (r *rowSync) set(header string, value any) {
	if r.row.New {
		r.row.Cells[header] = value
		return
	}
	from, written := r.read.Cells[header], r.exported.Cells[header]
	if inTab(from, value, written) {
		return
	}
	r.row.Cells[header] = value
	c := Change{Cell: r.cell(header), Row: describe(r.read), Header: header, From: from, To: value}
	if !isEmpty(from) && (written == nil || !sheet.SameValue(from, written)) {
		c.Check = fmt.Sprintf("the tab has %s, which mc didn't write", brief(from))
		if r.row.Check == nil {
			r.row.Check = map[string]string{}
		}
		r.row.Check[header] = c.Check
	}
	r.result.Changes = append(r.result.Changes, c)
}

// keep records that a cell of the row stays as it is in the tab, and why.
func (r *rowSync) keep(header string, value any, why string) {
	r.result.Changes = append(r.result.Changes, Change{
		Cell: r.cell(header), Row: describe(r.read), Header: header, From: r.read.Cells[header], To: value, Kept: why,
	})
}

// cell returns the address of the cell of the row under the header, such as Evaluation!G81.
func (r *rowSync) cell(header string) string {
	for letter, h := range sheetColumns {
		if h == header {
			return SheetTab + "!" + letter + strconv.Itoa(r.row.Number)
		}
	}
	return SheetTab + "!" + strconv.Itoa(r.row.Number)
}

// comment returns the TA Comment of the rule: what the team did well, what it can improve, and
// the reason of the penalty, if any.
func (r *rowSync) comment() string {
	parts := []string{"Done well: " + sentence(r.rule.DoneWell), "To improve: " + sentence(r.rule.ToImprove)}
	if p := r.rule.Penalty; p != nil && !blank(p.Why) {
		parts = append(parts, "Penalty: "+sentence(p.Why))
	}
	return strings.Join(parts, " ")
}

// inTab reports whether the cell of the tab holds the value already. The tab gives the result of a
// formula, not the formula: a formula is in the tab when mc wrote it last, and the cell is not
// empty.
func inTab(from, value, written any) bool {
	if isEmpty(from) {
		return false
	}
	if v, ok := value.(string); ok && strings.HasPrefix(v, "=") {
		w, ok := written.(string)
		return ok && w == v
	}
	return sheet.SameValue(from, value)
}

// rowsByKey returns the rows of a file by their keys.
func rowsByKey(f *sheet.File) map[string]sheet.Row {
	rows := make(map[string]sheet.Row, len(f.Rows))
	for _, row := range f.Rows {
		rows[rowKey(row)] = row
	}
	return rows
}

func rowKey(row sheet.Row) string {
	return key3(text(row.Cells[headerTeam]), text(row.Cells[headerLab]), text(row.Cells[headerRule]))
}

func key3(team, lab, rule string) string {
	return team + "\x1f" + lab + "\x1f" + rule
}

// describe returns the keys of a row, to name it in a message: teal L2 L2_POD.
func describe(row sheet.Row) string {
	return text(row.Cells[headerTeam]) + " " + text(row.Cells[headerLab]) + " " + text(row.Cells[headerRule])
}

// text returns the value of a cell as a text, empty for an empty cell.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return fmt.Sprint(v)
}

// brief returns the value of a cell for a message, quoted, and cut after 40 characters.
func brief(v any) string {
	if isEmpty(v) {
		return "nothing"
	}
	s := []rune(text(v))
	if len(s) > 40 {
		return strconv.Quote(string(s[:40]) + "...")
	}
	return strconv.Quote(string(s))
}

func isEmpty(v any) bool {
	return strings.TrimSpace(text(v)) == ""
}

// number returns the number of a cell, false if it is empty or not a number.
func number(v any) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(text(v)), 64)
	return f, err == nil
}

func completenessCell(x float64) json.Number {
	return json.Number(formatFloat(x))
}

// sentence returns s on one line, ending with a period unless it ends with another mark already.
func sentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return s
	}
	return s + "."
}

// joinSentences joins the texts that are not empty, each one a sentence.
func joinSentences(texts ...string) string {
	var parts []string
	for _, t := range texts {
		if t = sentence(t); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}
