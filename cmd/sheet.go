package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/sheet"
	"github.com/spf13/cobra"
)

var sheetDir string

var sheetCmd = &cobra.Command{
	Use:   "sheet",
	Short: "Move the data of a tab between ~/.mc/{year}/sheet/ and the evaluation spreadsheet",
	Long: `Move the data of a tab of the evaluation spreadsheet between JSON files and the spreadsheet,
by hand or through a browser. Each tab has its files in ~/.mc/{year}/sheet/, named after the
tab, such as evaluation for the tab Evaluation:

  evaluation.json           what waits to be written in the tab
  evaluation.read.json      the tab, as read in the browser by the script of mc sheet snippet
  evaluation.exported.json  what was written last, to tell a cell that the teacher changed

A file holds one object per row, whose attributes are the columns, named by their headers:

  {
    "tab": "Evaluation",
    "keys": ["Team", "Lab", "Rule"],
    "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "O": "TA Analysis"},
    "rows": [
      {"_row": 82, "Team": "teal", "Lab": "L2", "Rule": "L2_FBP", "Completeness": 0.9, "TA Analysis": "short",
       "_check": {"Completeness": "0.7 at the last export"}}
    ]
  }

The keys find a row, and "_row" is its number in the tab. A row holds the cells to write only:
an attribute absent is a cell left alone, never cleared. A column out of "columns" is never read
nor written. "_check" names the cells to look at before writing them, which the teacher may have
changed since the last export.`,
	Example: `  mc sheet snippet evaluation | pbcopy   # then run it in the page of the spreadsheet
  pbpaste > ~/.mc/2026/sheet/evaluation.read.json
  mc sheet blocks evaluation
  mc sheet copy evaluation 1             # then paste it in the tab, at its range
  mc sheet pasted evaluation
  mc sheet verify evaluation`,
}

var sheetSnippetCmd = &cobra.Command{
	Use:   "snippet TAB",
	Short: "Print the script that reads a tab in the browser, and copies its rows to the clipboard",
	Long: `Print the script that reads a tab in the page of the spreadsheet, the columns of its files only,
and copies the rows to the clipboard, as a file of the tab: save it as TAB.read.json. The
columns come from TAB.json, or else from TAB.exported.json. The script refuses to run in
another spreadsheet than the evaluation sheet of the team registry, if any.

gviz, which the script queries, types a column by the majority of its values: a value of
another type, such as a text in a column of numbers, comes back empty.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runSheetSnippet,
	SilenceUsage: true,
}

var sheetBlocksCmd = &cobra.Command{
	Use:   "blocks TAB",
	Short: "List the blocks of cells to paste to write TAB.json",
	Long: `List the blocks of cells to paste to write what waits in TAB.json: rectangles of adjacent cells,
every one of them written, so that a paste never clears a cell. The keys of the rows are never
written. The cells with a "_check" come last, in blocks of their own: look at them in the tab
before pasting them.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runSheetBlocks,
	SilenceUsage: true,
}

var sheetCopyPrint bool

var sheetCopyCmd = &cobra.Command{
	Use:   "copy TAB BLOCK",
	Short: "Copy a block of TAB.json to the clipboard, to paste it in the tab",
	Long: `Copy a block of TAB.json to the clipboard, as tab-separated values, which a paste spreads on
the cells of the block. Then type the range of the block in the Name Box of the spreadsheet,
press Enter, and paste. A text that Sheets would read as something else, such as the commit
2e88112, starts with an apostrophe, which keeps it a text.`,
	Args:         cobra.ExactArgs(2),
	RunE:         runSheetCopy,
	SilenceUsage: true,
}

var sheetPastedCmd = &cobra.Command{
	Use:   "pasted TAB [BLOCK...]",
	Short: "Record the blocks pasted as the last export, in TAB.exported.json",
	Long: `Record the blocks pasted in the tab as the last export: their cells move from TAB.json to
TAB.exported.json. Without a block, every block was pasted. The cells of a block not pasted,
such as one to check that the teacher left alone, wait still in TAB.json.`,
	Args:         cobra.MinimumNArgs(1),
	RunE:         runSheetPasted,
	SilenceUsage: true,
}

var sheetVerifyCmd = &cobra.Command{
	Use:   "verify TAB",
	Short: "Compare the tab, as read, with the last export, and exit with 1 on a difference",
	Long: `Compare the tab, as read in TAB.read.json, with the last export, TAB.exported.json: a header
that changed, a row that moved or that the tab no longer has, and a cell whose value is no
longer the one written, such as a cell that the teacher changed. A formula is never compared,
since the tab gives its result. Print one difference per line, and exit with 1 if there is any.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runSheetVerify,
	SilenceUsage: true,
}

func init() {
	sheetCmd.PersistentFlags().StringVar(&sheetDir, "dir", "", "The directory of the files of the tabs (default: ~/.mc/{year}/sheet)")
	sheetCopyCmd.Flags().BoolVar(&sheetCopyPrint, "print", false, "Print the block instead of copying it to the clipboard")
	sheetCmd.AddCommand(sheetSnippetCmd, sheetBlocksCmd, sheetCopyCmd, sheetPastedCmd, sheetVerifyCmd)
	RootCmd.AddCommand(sheetCmd)
}

// tabFiles are the files of a tab.
type tabFiles struct {
	pending, read, exported string
}

var tabFileName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// filesOf returns the files of the tab whose files are named name, such as evaluation.
func filesOf(name string) (tabFiles, error) {
	if !tabFileName.MatchString(name) {
		return tabFiles{}, fmt.Errorf("%q doesn't name the files of a tab: lowercase letters, digits and dashes, such as evaluation", name)
	}
	dir := sheetDir
	if dir == "" {
		dir = filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "sheet")
	}
	dir = common.ExpandHome(dir)
	return tabFiles{
		pending:  filepath.Join(dir, name+".json"),
		read:     filepath.Join(dir, name+".read.json"),
		exported: filepath.Join(dir, name+".exported.json"),
	}, nil
}

// loadIfExists loads a file of a tab, nil if it doesn't exist.
func loadIfExists(path string) (*sheet.File, error) {
	f, err := sheet.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return f, err
}

// loadPending loads TAB.json and its blocks.
func loadPending(name string) (tabFiles, *sheet.File, []sheet.Block, error) {
	files, err := filesOf(name)
	if err != nil {
		return files, nil, nil, err
	}
	f, err := loadIfExists(files.pending)
	if err != nil {
		return files, nil, nil, err
	}
	if f == nil {
		return files, nil, nil, fmt.Errorf("nothing waits to be written: no %s", files.pending)
	}
	blocks, err := sheet.Blocks(f)
	return files, f, blocks, err
}

func runSheetSnippet(cmd *cobra.Command, args []string) error {
	files, err := filesOf(args[0])
	if err != nil {
		return err
	}
	f, err := loadIfExists(files.pending)
	if err != nil {
		return err
	}
	if f == nil {
		if f, err = loadIfExists(files.exported); err != nil {
			return err
		}
	}
	if f == nil {
		return fmt.Errorf("no columns to read: neither %s nor %s exists", files.pending, files.exported)
	}
	var url string
	if registry, err := common.LoadRegistry(); err == nil {
		url = registry.EvaluationSheet
	}
	return sheet.Snippet(cmd.OutOrStdout(), f, sheet.SnippetOptions{Sheet: url, Path: files.read})
}

func runSheetBlocks(cmd *cobra.Command, args []string) error {
	files, f, blocks, err := loadPending(args[0])
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(blocks) == 0 {
		fmt.Fprintf(out, "Nothing waits to be written in %s.\n", files.pending)
		return nil
	}
	fmt.Fprintf(out, "%s of the tab %s %s in %s:\n\n", count(len(blocks), "block"), f.Tab, verb(len(blocks), "waits", "wait"), files.pending)
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "BLOCK\tRANGE\tSIZE\tCHECK FIRST")
	for i, b := range blocks {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", i+1, b.Range(), b.Size(), strings.Join(b.Check, "; "))
	}
	return w.Flush()
}

func runSheetCopy(cmd *cobra.Command, args []string) error {
	_, _, blocks, err := loadPending(args[0])
	if err != nil {
		return err
	}
	i, err := strconv.Atoi(args[1])
	if err != nil || i < 1 || i > len(blocks) {
		return fmt.Errorf("%q is not a block: from 1 to %d", args[1], len(blocks))
	}
	b := blocks[i-1]
	out, message := cmd.OutOrStdout(), cmd.ErrOrStderr()
	for _, why := range b.Check {
		fmt.Fprintf(message, "Check first: %s\n", why)
	}
	if sheetCopyPrint {
		fmt.Fprintf(message, "Block %d of %d, %s (%s):\n", i, len(blocks), b.Range(), b.Size())
		_, err := io.WriteString(out, b.TSV())
		return err
	}
	if err := toClipboard(b.TSV()); err != nil {
		return err
	}
	fmt.Fprintf(out, "Copied the block %d of %d to the clipboard: type %s in the Name Box, press Enter, then paste.\n", i, len(blocks), b.Range())
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
	return errors.New("no pbcopy, wl-copy nor xclip to copy to the clipboard: pass --print, and copy the block printed")
}

func runSheetPasted(cmd *cobra.Command, args []string) error {
	files, f, blocks, err := loadPending(args[0])
	if err != nil {
		return err
	}
	pasted := blocks
	if len(args) > 1 {
		pasted = nil
		for _, arg := range args[1:] {
			i, err := strconv.Atoi(arg)
			if err != nil || i < 1 || i > len(blocks) {
				return fmt.Errorf("%q is not a block: from 1 to %d", arg, len(blocks))
			}
			pasted = append(pasted, blocks[i-1])
		}
	}
	exported, err := loadIfExists(files.exported)
	if err != nil {
		return err
	}
	if exported == nil {
		exported = sheet.NewExport(f)
	}
	cells, err := sheet.RecordPasted(f, exported, pasted)
	if err != nil {
		return fmt.Errorf("%s and %s: %w", files.pending, files.exported, err)
	}
	if err := exported.Save(files.exported); err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Recorded %s of %s as the last export, in %s.\n", count(cells, "cell"), count(len(pasted), "block"), files.exported)
	if len(f.Rows) == 0 {
		if err := os.Remove(files.pending); err != nil {
			return err
		}
		fmt.Fprintln(out, "Nothing waits to be written anymore.")
		return nil
	}
	if err := f.Save(files.pending); err != nil {
		return err
	}
	left := len(blocks) - len(pasted)
	fmt.Fprintf(out, "%s %s still in %s: mc sheet blocks %s.\n", count(left, "block"), verb(left, "waits", "wait"), files.pending, args[0])
	return nil
}

func runSheetVerify(cmd *cobra.Command, args []string) error {
	files, err := filesOf(args[0])
	if err != nil {
		return err
	}
	read, err := sheet.Load(files.read)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("no %s: read the tab first, with the script of mc sheet snippet %s", files.read, args[0])
	}
	if err != nil {
		return err
	}
	exported, err := sheet.Load(files.exported)
	if err != nil {
		return err
	}
	differences, err := sheet.Verify(read, exported)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	for _, d := range differences {
		fmt.Fprintln(out, d)
	}
	if len(differences) > 0 {
		return fmt.Errorf("%d differences between %s and %s", len(differences), files.read, files.exported)
	}
	fmt.Fprintf(out, "The tab %s holds the last export: %d rows.\n", exported.Tab, len(exported.Rows))
	return nil
}

// count returns a number of things, such as "1 block" or "2 blocks".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return strconv.Itoa(n) + " " + thing + "s"
}

// verb returns the verb that agrees with a number of things.
func verb(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
