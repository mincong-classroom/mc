package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/rubric"
)

var ruleSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Compute the cells that write the evaluation rules of the year in the evaluation sheet",
	Long: `Compute the cells that write the evaluation rules of the year in the tab Rules of the evaluation
sheet, where the tab Evaluation looks up the score of each rule, and save them in
~/.mc/{year}/sheet/rules.json, the file of mc sheet. mc has no access to the spreadsheet: an agent
in the browser, such as Claude in Chrome, reads the tab and pastes the blocks.

The rules are checked first, as by mc rule check, but for the files of the teacher. Then the sync
needs the tab as read after the last paste, rules.read.json: without it, or when it is older than
the last export, the sync writes the script that reads the tab, rules.read.js, and stops.

  - A row is found by its Lab Session and its Rule, the end of the Rule ID: L2 and FBP for L2_FBP.
    A rule that the tab doesn't have gets a new row after the last one, with the formulas of the
    Rule ID, the Score and the Exercise ID.
  - The Rule Name, the Coefficient, the Exercise and the Rule Description follow the YAML, the
    description on one line. A cell that mc didn't write, such as a coefficient tuned in the tab,
    waits in a block to check first.
  - A formula whose value is not the one that it gives, such as a Rule ID typed over it, is written
    again, in a block to check first.
  - A row whose rule the YAML no longer has is listed, never deleted.
  - The column J, Troubleshooting Instructions, is never read nor written.`,
	Example: `  mc rule sync
  mc rule sync --rules-file evaluation-rules.yaml`,
	Args:         cobra.NoArgs,
	RunE:         runRuleSync,
	SilenceUsage: true,
}

func init() {
	ruleSyncCmd.Flags().StringVar(&sheetDir, "sheet-dir", "", "The directory of the files of the tabs (default: ~/.mc/{year}/sheet)")
	ruleCmd.AddCommand(ruleSyncCmd)
}

func runRuleSync(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(ruleFile)
	if err != nil {
		return err
	}
	// The content of the rules only: the files of the teacher are not in the tab.
	r, err := loadRules(path, rubric.CheckOptions{}, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	tab, err := filesOf("rules")
	if err != nil {
		return err
	}
	read, err := readSinceExport(cmd.OutOrStdout(), tab, evaluation.RulesSheetFile(), "mc rule sync")
	if err != nil {
		return err
	}
	exported, err := loadIfExists(tab.exported)
	if err != nil {
		return err
	}
	result, err := evaluation.SyncRules(evaluation.RulesSyncOptions{Rules: r.Rules, Read: read, Exported: exported})
	if err != nil {
		return err
	}
	if err := savePending(tab.pending, result.Pending); err != nil {
		return err
	}
	return printRuleSync(cmd.OutOrStdout(), ruleSyncReport{rules: len(r.Rules), path: path, tab: tab, result: result})
}

// ruleSyncReport is what printRuleSync prints.
type ruleSyncReport struct {
	rules  int    // The number of rules of the YAML.
	path   string // The YAML of the rules.
	tab    tabFiles
	result *evaluation.RulesSyncResult
}

// printRuleSync prints the cells that a sync of the rules writes, its new rows, the rows of the tab
// that the YAML no longer has, and what to do next.
func printRuleSync(w io.Writer, s ruleSyncReport) error {
	result := s.result
	fmt.Fprintf(w, "Synced %s of %s with the tab Rules, as read in %s.\n", plural(s.rules, "rule"), s.path, s.tab.read)
	if err := printChanges(w, "RULE", result.Changes); err != nil {
		return err
	}
	if len(result.Added) > 0 {
		fmt.Fprintf(w, "\nNew %s: %s.\n", plural(len(result.Added), "row"), ruleRows(result.Added))
	}
	if len(result.Extra) > 0 {
		fmt.Fprintf(w, "\nIn the tab, not in the rules: %s. mc never deletes a row: delete it by hand if the rule is gone.\n", ruleRows(result.Extra))
	}
	if len(result.Pending.Rows) == 0 {
		fmt.Fprintf(w, "\nNothing to write: the tab holds the rules already.\n")
		return nil
	}
	return printNextSteps(w, nextSteps{name: "rules", rows: len(result.Pending.Rows), pending: s.tab.pending})
}

// ruleRows returns the rules and their rows, such as "L2_FBP (row 5), L2_POD (row 6)".
func ruleRows(rows []evaluation.RuleRow) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = r.ID + " (row " + strconv.Itoa(r.Row) + ")"
	}
	return strings.Join(parts, ", ")
}
