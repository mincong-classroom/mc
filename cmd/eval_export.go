package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

var (
	evalExportLabs  []string
	evalExportTeams []string
	evalExportDir   string
)

var evalExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Write the export of the evaluation sheet, and copy it to the clipboard to import it",
	Long: `Write the export of the evaluation sheet, ~/.mc/{year}/sheet/export.json, and copy it to the
clipboard: the tab Rules from the YAML of the rules, and the tab Evaluation from the evaluations of
~/.mc/{year}/evaluations/. In the spreadsheet, the menu "mc", then "Import", imports it: the
script of mc sheet script finds each row by its keys, then rewrites the columns of mc on every row
but the confirmed ones. It never writes the other columns: B and C, the students, and H, J and K,
the teacher's.

The files of each lab are checked first, as by mc eval run --check-only. Then the export replaces
the cells of the last export with the evaluations:

  - A row is found by its team, its lab and its rule. A missing row is added after the last row.
  - Every TA column follows the evaluation: the TA Completeness, even when lower than the last
    export, with its TA Comment, what the team did well and what it can improve. A text that the
    evaluation leaves empty clears its cell.
  - TA Docker Images and TA Screenshots follow the facts of mc eval gather-facts, the same way:
    the tags of the images of the rule, and the number of screenshots of the team.
  - A penalty becomes the formula of TA Penalty, such as =-0.1*IF(ISBLANK(H{row}),G{row},H{row})*F{row},
    and its reason goes to the TA Comment. Without a penalty, the cell is cleared.
  - The Score takes the completeness of the teacher, in H, when the teacher filled it. A filled H
    confirms the row: the teacher reviewed the evaluation of the rule, and it is final. The import
    never writes a confirmed row, and its summary lists the cells that differ from the export.

The export prints each cell that changed since the last export, and counts the completenesses
that went down: check them before the import.

Without --lab, every lab with evaluations is exported. The rows of the other labs and teams stay
as they are in the last export.`,
	Example: `  mc eval export
  mc eval export -l L2 -t teal`,
	Args:         cobra.NoArgs,
	RunE:         runEvalExport,
	SilenceUsage: true,
}

func init() {
	evalExportCmd.Flags().StringArrayVarP(&evalExportLabs, "lab", "l", nil, "A lab, e.g. L2 or 2, repeatable (default: every lab with evaluations)")
	evalExportCmd.Flags().StringArrayVarP(&evalExportTeams, "team", "t", nil, "A team, repeatable (default: every team with a file)")
	evalExportCmd.Flags().StringVar(&evalExportDir, "sheet-dir", "", "The directory of export.json (default: ~/.mc/{year}/sheet)")
	evalCmd.AddCommand(evalExportCmd)
}

func runEvalExport(cmd *cobra.Command, _ []string) error {
	stderr := cmd.ErrOrStderr()
	r, path, err := loadEvalRules(stderr)
	if err != nil {
		return err
	}
	labs, err := exportedLabs(r, path)
	if err != nil {
		return err
	}
	var evaluations []evaluation.LabEvaluations
	files := 0
	for _, lab := range labs {
		e, err := labEvaluations(stderr, r, lab)
		if err != nil {
			return err
		}
		if len(e.Files) == 0 {
			continue
		}
		files += len(e.Files)
		evaluations = append(evaluations, e)
	}
	if len(evalExportTeams) > 0 && files == 0 {
		return fmt.Errorf("no evaluation of %s", strings.Join(evalExportTeams, ", "))
	}

	file := exportFile()
	last, err := sheet.LoadExport(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		last = nil
	case err != nil:
		return err
	}
	var spreadsheet string
	if registry, err := common.LoadRegistry(); err == nil {
		spreadsheet = sheet.SpreadsheetID(registry.EvaluationSheet)
	}
	result, err := evaluation.Export(evaluation.ExportOptions{
		Spreadsheet: spreadsheet, Rules: r.Rules, Labs: evaluations, Last: last,
	})
	if err != nil {
		return err
	}
	if err := result.Export.Save(file); err != nil {
		return err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if err := printExport(out, exportReport{file: file, last: last != nil, evaluations: evaluations, result: result}); err != nil {
		return err
	}
	if err := toClipboard(string(data)); err != nil {
		fmt.Fprintf(out, "\nCould not copy the export to the clipboard (%v): copy the content of %s instead.\n", err, file)
	} else {
		fmt.Fprintf(out, "\nCopied the export to the clipboard, %d KB.\n", (len(data)+1023)/1024)
	}
	fmt.Fprint(out, `Next, in the spreadsheet: the menu "mc", then "Import", paste it in the text box, then Import.
The first time, add the script of mc sheet script: "Extensions", then "Apps Script".
`)
	return nil
}

// exportedLabs returns the labs of --lab, or else every lab of the rules.
func exportedLabs(r *rubric.Rubric, path string) ([]rubric.Lab, error) {
	if len(evalExportLabs) == 0 {
		return r.Labs, nil
	}
	var labs []rubric.Lab
	for _, id := range evalExportLabs {
		lab, ok := r.Lab(rubric.LabID(id))
		if !ok {
			return nil, fmt.Errorf("no lab %q in %s", id, path)
		}
		labs = append(labs, lab)
	}
	return labs, nil
}

// labEvaluations returns the evaluations of a lab, checked: those of --team, or of every team with
// a file. A lab without a file has none, unless --lab names it.
func labEvaluations(w io.Writer, r *rubric.Rubric, lab rubric.Lab) (evaluation.LabEvaluations, error) {
	e := evaluation.LabEvaluations{Lab: lab.ID}
	labDir := evaluation.LabDir(evaluationsDir(), lab)
	names, err := evaluation.Teams(labDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return e, err
	}
	if len(evalExportTeams) > 0 {
		names = slices.DeleteFunc(names, func(name string) bool { return !slices.Contains(evalExportTeams, name) })
	}
	if len(names) == 0 {
		if len(evalExportLabs) > 0 && len(evalExportTeams) == 0 {
			return e, fmt.Errorf("no evaluation of %s in %s", lab.ID, labDir)
		}
		return e, nil
	}
	if err := checkEvaluations(w, r, lab, labDir, names); err != nil {
		return e, fmt.Errorf("%w: fix the files first, then mc eval run -l %s --check-only", err, lab.ID)
	}
	for _, name := range names {
		f, err := evaluation.Load(evaluation.Path(labDir, name))
		if err != nil {
			return e, err
		}
		e.Files = append(e.Files, f)
	}
	if e.Facts, err = readFacts(lab, names); err != nil {
		return e, err
	}
	return e, nil
}

// exportFile returns the path of the export: in --sheet-dir, or in the directory of the year.
func exportFile() string {
	dir := evalExportDir
	if dir == "" {
		dir = filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "sheet")
	}
	return filepath.Join(common.ExpandHome(dir), "export.json")
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

// exportReport is what printExport prints.
type exportReport struct {
	file        string
	last        bool // Whether there was a last export.
	evaluations []evaluation.LabEvaluations
	result      *evaluation.ExportResult
}

// printExport prints what the export holds, the cells that changed since the last export, and its
// new rows.
func printExport(w io.Writer, r exportReport) error {
	export := r.result.Export
	rules, evaluations := export.Tab(evaluation.RulesTab), export.Tab(evaluation.SheetTab)
	labs := make([]string, len(r.evaluations))
	files := 0
	for i, e := range r.evaluations {
		labs[i] = e.Lab
		files += len(e.Files)
	}
	from := "none"
	if len(labs) > 0 {
		from = plural(files, "evaluation") + " of " + strings.Join(labs, ", ")
	}
	fmt.Fprintf(w, "Exported %s and %s to %s, from %s.\n",
		plural(len(rules.Rows), "rule"), plural(len(evaluations.Rows), "row")+" of the tab Evaluation", r.file, from)
	if !r.last {
		fmt.Fprintln(w, "No last export: every row is new.")
		return nil
	}
	if len(r.result.Changes) > 0 {
		fmt.Fprintln(w)
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TAB\tROW\tCOLUMN\tCHANGE")
		lower := 0
		for _, c := range r.result.Changes {
			change := cellValue(c.From) + " -> " + cellValue(c.To)
			if c.Lower() {
				lower++
				change += ", lower"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Tab, c.Row, c.Header, change)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		if lower > 0 {
			noun := "completenesses"
			if lower == 1 {
				noun = "completeness"
			}
			fmt.Fprintf(w, "\n%d %s lower than the last export, to check before the import.\n", lower, noun)
		}
	}
	for _, tab := range []string{evaluation.RulesTab, evaluation.SheetTab} {
		added := r.result.Added[tab]
		if len(added) == 0 {
			continue
		}
		shown := added
		if len(shown) > 20 {
			shown = shown[:20]
		}
		more := ""
		if len(added) > len(shown) {
			more = fmt.Sprintf(", and %d more", len(added)-len(shown))
		}
		fmt.Fprintf(w, "\nNew %s of the tab %s: %s%s.\n", plural(len(added), "row"), tab, strings.Join(shown, ", "), more)
	}
	if len(r.result.Removed) > 0 {
		fmt.Fprintf(w, "\nIn the last export, not in the rules: %s. The import keeps their rows, cleared but for their keys: delete them by hand.\n",
			strings.Join(r.result.Removed, ", "))
	}
	if len(r.result.Changes) == 0 && len(r.result.Added[evaluation.RulesTab])+len(r.result.Added[evaluation.SheetTab]) == 0 {
		fmt.Fprintln(w, "No change since the last export.")
	}
	return nil
}

// cellValue returns the value of a cell for a line of the report: a number as it is, and a text
// quoted and cut after 40 characters.
func cellValue(v any) string {
	switch v := v.(type) {
	case nil:
		return "nothing"
	case json.Number:
		return v.String()
	case string:
		r := []rune(v)
		if len(r) > 40 {
			return strconv.Quote(string(r[:40]) + "...")
		}
		return strconv.Quote(v)
	}
	return fmt.Sprint(v)
}
