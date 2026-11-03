package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/sheet"
)

var sheetCmd = &cobra.Command{
	Use:   "sheet",
	Short: "Set up the evaluation spreadsheet, which imports the export of mc eval export",
	Long: `Set up the evaluation spreadsheet, which imports the export of mc eval export with a script of
its own, written with Apps Script. mc has no access to the spreadsheet: the teacher, or an agent in
the browser, pastes the export in the dialog of the script.`,
}

var sheetScriptCopy bool

var sheetScriptCmd = &cobra.Command{
	Use:   "script",
	Short: "Print the script that imports the export of mc eval export in the spreadsheet",
	Long: `Print the script that imports the export of mc eval export in the spreadsheet. Paste it once in
"Extensions", then "Apps Script", in place of the code there, save it, then reload the
spreadsheet: the menu "mc", then "Import", opens a dialog, where you paste the export. The first
import asks to authorize the script, once, for this spreadsheet only.

The script finds each row by its keys, such as the team, the lab and the rule, and a new key gets
a new row at the end, whose students it copies from the row above. Then it rewrites the columns of
mc on every row, reads them back, and compares them with the export. It stops before any write if a
header of the export is not in the first row of its tab. It never writes the other columns, and
reads none of them but the keys.`,
	Example:      `  mc sheet script --copy`,
	Args:         cobra.NoArgs,
	RunE:         runSheetScript,
	SilenceUsage: true,
}

func init() {
	sheetScriptCmd.Flags().BoolVar(&sheetScriptCopy, "copy", false, "Copy the script to the clipboard instead of printing it")
	sheetCmd.AddCommand(sheetScriptCmd)
	RootCmd.AddCommand(sheetCmd)
}

func runSheetScript(cmd *cobra.Command, _ []string) error {
	script := sheet.ImportScript()
	if !sheetScriptCopy {
		_, err := fmt.Fprint(cmd.OutOrStdout(), script)
		return err
	}
	if err := toClipboard(script); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Copied the script to the clipboard, %d lines: paste it in \"Extensions\", then \"Apps Script\".\n",
		strings.Count(script, "\n"))
	return nil
}

// toClipboard copies a text to the clipboard, with the command of the system.
func toClipboard(text string) error {
	for _, c := range [][]string{{"pbcopy"}, {"wl-copy"}, {"xclip", "-selection", "clipboard"}} {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		copier := exec.Command(c[0], c[1:]...)
		copier.Stdin = strings.NewReader(text)
		copier.Stderr = os.Stderr
		return copier.Run()
	}
	return errors.New("no pbcopy, wl-copy nor xclip to copy to the clipboard")
}
