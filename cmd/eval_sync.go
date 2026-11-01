package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

var (
	evalSyncLab   string
	evalSyncTeams []string
)

var evalSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Compute the cells that write the evaluations of a lab in the evaluation sheet",
	Long: `Compute the cells that write the evaluations of a lab in the tab Evaluation of the evaluation
sheet, and save them in ~/.mc/{year}/sheet/evaluation.json, the file of mc sheet. mc has no access
to the spreadsheet: an agent in the browser, such as Claude in Chrome, reads the tab and pastes the
blocks. Without one, the evaluations stay in their files, and nothing is written.

The files of the lab are checked first, as by mc eval run --check-only. Then the sync needs the
tab as read after the last paste, evaluation.read.json: without it, or when it is older than the
last export, the sync writes the script that reads the tab, evaluation.read.js, and stops.

The sync follows the rules of a pass:

  - A row is found by its team, its lab and its rule. A missing row is added after the last row
    of the tab, with its keys and its formulas. The B and C of the last row of the tab are then
    filled down to the new rows.
  - The completeness goes to an empty cell, or over the one that mc wrote last when it is higher,
    with its TA Comment: what the team did well, and what it can improve. A lower one keeps the
    former value, and so does a completeness that mc didn't write, such as one of the teacher:
    TA To Check says why.
  - The other TA columns follow the evaluation, but a cell is never cleared. A cell that mc didn't
    write, and that the evaluation changes, waits in a block to check first.
  - TA Docker Images and TA Screenshots follow the facts of mc eval gather-facts, the same way:
    the tags of the images of the rule, and the number of screenshots of the team.
  - A penalty becomes the formula of TA Penalty, such as =-0.1*G80*F80, and its reason goes to
    the TA Comment.

The rows of the lab and the teams that waited in evaluation.json are replaced, and the others
wait still.`,
	Example: `  mc eval sync -l L2
  mc eval sync -l 2 -t teal`,
	Args:         cobra.NoArgs,
	RunE:         runEvalSync,
	SilenceUsage: true,
}

func init() {
	evalSyncCmd.Flags().StringVarP(&evalSyncLab, "lab", "l", "", "The lab, e.g. L2 or 2")
	evalSyncCmd.Flags().StringArrayVarP(&evalSyncTeams, "team", "t", nil, "A team, repeatable (default: every team with a file)")
	evalSyncCmd.Flags().StringVar(&sheetDir, "sheet-dir", "", "The directory of the files of the tabs (default: ~/.mc/{year}/sheet)")
	_ = evalSyncCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	evalCmd.AddCommand(evalSyncCmd)
}

// errNoRead says that the tab must be read before a sync.
var errNoRead = errors.New("read the tab first")

func runEvalSync(cmd *cobra.Command, _ []string) error {
	r, path, err := loadEvalRules(cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	lab, ok := r.Lab(rubric.LabID(evalSyncLab))
	if !ok {
		return fmt.Errorf("no lab %q in %s", evalSyncLab, path)
	}
	labDir := evaluation.LabDir(evaluationsDir(), lab)
	names, err := labTeams(labDir, lab, evalSyncTeams)
	if err != nil {
		return err
	}
	if err := checkEvaluations(cmd.ErrOrStderr(), r, lab, labDir, names); err != nil {
		return fmt.Errorf("%w: fix the files first, then mc eval run -l %s --check-only", err, lab.ID)
	}
	files := make([]*evaluation.File, len(names))
	for i, name := range names {
		if files[i], err = evaluation.Load(evaluation.Path(labDir, name)); err != nil {
			return err
		}
	}

	tab, err := filesOf("evaluation")
	if err != nil {
		return err
	}
	read, err := readSinceExport(cmd.OutOrStdout(), tab, evaluation.SheetFile(), "mc eval sync -l "+lab.ID)
	if err != nil {
		return err
	}
	exported, err := loadIfExists(tab.exported)
	if err != nil {
		return err
	}
	pending, err := loadIfExists(tab.pending)
	if err != nil {
		return err
	}
	facts, err := readFacts(lab, names)
	if err != nil {
		return err
	}
	result, err := evaluation.Sync(evaluation.SyncOptions{
		Lab: lab.ID, Files: files, Read: read, Exported: exported, Pending: pending, Facts: facts,
	})
	if err != nil {
		return err
	}
	if err := savePending(tab.pending, result.Pending); err != nil {
		return err
	}
	return printSync(cmd.OutOrStdout(), syncReport{lab: lab, teams: len(files), tab: tab, result: result})
}

// readFacts returns the facts of mc for the columns that the evaluator doesn't write, by team, then
// by rule. The checks of the files ensure that the facts are at the commit of each evaluation.
func readFacts(lab rubric.Lab, names []string) (map[string]map[string]evaluation.RuleFacts, error) {
	teams, err := common.ListTeams()
	if err != nil {
		return nil, err
	}
	facts := make(map[string]map[string]evaluation.RuleFacts, len(names))
	for _, name := range names {
		team, _ := teamOf(teams, name) // The checks report a team that the registry doesn't have.
		f, ok, err := evidence.ReadFacts(factsOf(teacherDir(), team, lab))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		rules := make(map[string]evaluation.RuleFacts, len(f.Rules))
		for id, rf := range f.Rules {
			rules[id] = evaluation.RuleFacts{Images: rf.Images, Screenshots: rf.Screenshots}
		}
		facts[name] = rules
	}
	return facts, nil
}

// readSinceExport returns the tab as read since the last export. Without such a read, it writes the
// script that reads the columns of the layout, says how to run it, then the command to run again,
// and returns errNoRead.
func readSinceExport(w io.Writer, tab tabFiles, layout *sheet.File, again string) (*sheet.File, error) {
	info, err := os.Stat(tab.read)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		exported, err := os.Stat(tab.exported)
		switch {
		case errors.Is(err, fs.ErrNotExist) || err == nil && !info.ModTime().Before(exported.ModTime()):
			return sheet.Load(tab.read)
		case err != nil:
			return nil, err
		}
	}
	script := tab.read[:len(tab.read)-len(".json")] + ".js"
	var url string
	if registry, err := common.LoadRegistry(); err == nil {
		url = registry.EvaluationSheet
	}
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(script, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	if err := sheet.Snippet(f, layout, sheet.SnippetOptions{Sheet: url, Path: tab.read}); err != nil {
		_ = f.Close() // The error of the script says more.
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	fmt.Fprintf(w, `No read of the tab %s since the last export, in %s. To read it, run the script
  %s
in the page of the spreadsheet, with an agent in the browser such as Claude in Chrome, or in the
console of its DevTools. It copies the rows to the clipboard: save them in %s, such as with
  pbpaste > %s
then run %s again.
`, layout.Tab, tab.read, script, tab.read, tab.read, again)
	return nil, errNoRead
}

// savePending saves what waits to be written, or removes its file when nothing waits.
func savePending(path string, f *sheet.File) error {
	if len(f.Rows) > 0 {
		return f.Save(path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// syncReport is what printSync prints.
type syncReport struct {
	lab    rubric.Lab
	teams  int
	tab    tabFiles
	result *evaluation.SyncResult
}

// printSync prints the cells that a sync writes and keeps, its new rows, and what to do next.
func printSync(w io.Writer, s syncReport) error {
	result := s.result
	fmt.Fprintf(w, "Synced %s of %s with the tab Evaluation, as read in %s.\n", plural(s.teams, "evaluation"), s.lab.ID, s.tab.read)
	if err := printChanges(w, "ROW", result.Changes); err != nil {
		return err
	}
	if len(result.Added) > 0 {
		fmt.Fprintln(w)
		for _, a := range result.Added {
			rows := "row " + strconv.Itoa(a.First)
			if a.Last > a.First {
				rows = fmt.Sprintf("rows %d to %d", a.First, a.Last)
			}
			fmt.Fprintf(w, "New %s: %s, %s.\n", plural(a.Last-a.First+1, "row"), a.Team, rows)
		}
	}
	if result.Replaced > 0 {
		fmt.Fprintf(w, "\nReplaced %s of %s that waited.\n", plural(result.Replaced, "row"), s.lab.ID)
	}
	if len(result.Pending.Rows) == 0 {
		fmt.Fprintf(w, "\nNothing to write: the tab holds the evaluations already.\n")
		return nil
	}
	var fill string
	if result.LastNewRow > 0 && result.FillFrom > 0 {
		fill = fmt.Sprintf("B%d:C%d", result.FillFrom, result.LastNewRow)
	}
	return printNextSteps(w, nextSteps{name: "evaluation", rows: len(result.Pending.Rows), pending: s.tab.pending, fill: fill})
}

// printChanges prints the cells that a sync writes and keeps, if any, one per line, after a blank
// line. row is the header of the column that names the row of each cell.
func printChanges(w io.Writer, row string, changes []evaluation.Change) error {
	if len(changes) == 0 {
		return nil
	}
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "CELL\t%s\tCOLUMN\tCHANGE\n", row)
	for _, c := range changes {
		change := cellValue(c.From) + " -> " + cellValue(c.To)
		switch {
		case c.Kept != "" && c.To == nil:
			change = "kept " + cellValue(c.From) + ": " + c.Kept
		case c.Kept != "":
			change = "kept " + cellValue(c.From) + ", not " + cellValue(c.To) + ": " + c.Kept
		case c.Check != "":
			change += ", check first: " + c.Check
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Cell, c.Row, c.Header, change)
	}
	return tw.Flush()
}

// nextSteps are the rows that wait in the file of a tab, and how to write them.
type nextSteps struct {
	name    string // The name of the files of the tab, such as evaluation.
	rows    int    // The number of rows that wait.
	pending string // The file of what waits.
	fill    string // The range to fill down after the paste, such as B3:C4, if any.
}

// printNextSteps prints the rows that wait, and the commands that write them in the tab through the
// page of the spreadsheet.
func printNextSteps(w io.Writer, s nextSteps) error {
	fmt.Fprintf(w, "\n%s %s in %s. Next, in the page of the spreadsheet:\n\n", plural(s.rows, "row"), verb(s.rows, "waits", "wait"), s.pending)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "  mc sheet blocks %s\tlist the blocks to paste\n", s.name)
	fmt.Fprintf(tw, "  mc sheet copy %s 1\tcopy a block, then paste it at its range, block after block\n", s.name)
	if s.fill != "" {
		fmt.Fprintf(tw, "  fill down %s\tthe columns B and C of the new rows: type the range in the Name Box, then Cmd+D\n", s.fill)
	}
	fmt.Fprintf(tw, "  mc sheet pasted %s\trecord the blocks pasted, such as 1 2 3, or every block\n", s.name)
	fmt.Fprintf(tw, "  mc sheet verify %s\tafter a new read of the tab, compare it with what was written\n", s.name)
	return tw.Flush()
}

// cellValue returns the value of a cell for a line of the report: a number as it is, even one
// read as a text, and a text quoted and cut after 40 characters.
func cellValue(v any) string {
	switch v := v.(type) {
	case nil:
		return "nothing"
	case json.Number:
		return v.String()
	case string:
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			return v
		}
		r := []rune(v)
		if len(r) > 40 {
			return strconv.Quote(string(r[:40]) + "...")
		}
		return strconv.Quote(v)
	}
	return fmt.Sprint(v)
}
