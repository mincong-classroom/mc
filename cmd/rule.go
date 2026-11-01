package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/rubric"
	"github.com/spf13/cobra"
)

// rulesFileUsage is the usage of the flag --rules-file, which replaces the rules of the year.
const rulesFileUsage = "The YAML of the evaluation rules, e.g. a test file (default: the key rules of ~/.mc/settings.yaml, for the year)"

var ruleFile string

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "List, check and render the evaluation rules of the year",
	Long: `List, check and render the evaluation rules of the year, in a YAML file. The file is the key
rules of ~/.mc/settings.yaml, where {year} is the year of the cohort:

  year: 2026
  rules: ~/github/my-org/course/{year}/evaluation-rules.yaml

The flag --rules-file replaces it, e.g. with a test file.`,
	Example: `  mc rule ls -l L1
  mc rule check
  mc rule render -o evaluation-rules.md`,
}

var ruleLsLab string

var ruleLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List the evaluation rules of the year",
	Long: `List the evaluation rules of the year: one line per rule, with its exercise, its coefficient
and its name.`,
	Args:         cobra.NoArgs,
	RunE:         runRuleLs,
	SilenceUsage: true,
}

var ruleCheckRoot string

var ruleCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the YAML of the evaluation rules, and exit with 1 on a problem",
	Long: `Check the YAML of the evaluation rules: the keys, the IDs of the labs and the rules, the
exercises, the coefficients (by steps of 0.05, those of the mandatory rules of a lab add up to 1),
the penalties, the levels of analysis, the kinds of evidence, the dependencies, the deadlines, the
paths, and the width of the lines (120 characters at most).

Print one problem per line, with the line of the file, and exit with 1 if there is any.`,
	Args:         cobra.NoArgs,
	RunE:         runRuleCheck,
	SilenceUsage: true,
}

var (
	ruleRenderOutput string
	ruleRenderRoot   string
)

var ruleRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render the evaluation rules of the YAML as a Markdown page",
	Long: `Check the YAML of the evaluation rules, then render it as a Markdown page: the labs, how the
score works, the evidence, the observations, then each lab with the table of its rules and each
rule. The texts are wrapped at 96 characters.`,
	Args:         cobra.NoArgs,
	RunE:         runRuleRender,
	SilenceUsage: true,
}

// maxRuleLineWidth is the width of a line of the YAML, so that it reads without scrolling in a
// browser.
const maxRuleLineWidth = 120

const rootFlagUsage = "The repository of the rules, against which the paths of the teacher are checked (default: the Git repository of the file)"

func init() {
	ruleCmd.PersistentFlags().StringVar(&ruleFile, "rules-file", "", rulesFileUsage)
	ruleLsCmd.Flags().StringVarP(&ruleLsLab, "lab", "l", "", "List the rules of one lab session only, e.g. L1 or 1")
	ruleCheckCmd.Flags().StringVar(&ruleCheckRoot, "root", "", rootFlagUsage)
	ruleRenderCmd.Flags().StringVarP(&ruleRenderOutput, "output", "o", "", "The Markdown file to write (default: the standard output)")
	ruleRenderCmd.Flags().StringVar(&ruleRenderRoot, "root", "", rootFlagUsage)
	ruleCmd.AddCommand(ruleLsCmd, ruleCheckCmd, ruleRenderCmd)
}

func runRuleLs(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(ruleFile)
	if err != nil {
		return err
	}
	r, err := rubric.Load(path)
	if err != nil {
		return err
	}
	labID := rubric.LabID(ruleLsLab)
	if labID != "" {
		if _, ok := r.Lab(labID); !ok {
			return fmt.Errorf("no lab %q in %s", ruleLsLab, path)
		}
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RULE\tEXERCISE\tCOEFFICIENT\tNAME")
	for _, rule := range r.Rules {
		if labID != "" && rule.Lab != labID {
			continue
		}
		name := rule.Name
		if rule.Optional() {
			name += ", bonus"
		}
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", rule.ID, rule.Exercise, rule.Coefficient, name)
	}
	return w.Flush()
}

func runRuleCheck(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(ruleFile)
	if err != nil {
		return err
	}
	_, err = loadAndCheck(path, ruleCheckRoot, cmd.ErrOrStderr())
	return err
}

func runRuleRender(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(ruleFile)
	if err != nil {
		return err
	}
	r, err := loadAndCheck(path, ruleRenderRoot, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	opts := rubric.RenderOptions{Source: filepath.Base(path)}
	if ruleRenderOutput != "" {
		// The links of the page are relative to the directory of the page.
		dir := filepath.Dir(ruleRenderOutput)
		if rel, err := filepath.Rel(dir, path); err == nil {
			opts.Source = filepath.ToSlash(rel)
		}
		root := ruleRenderRoot
		if root == "" {
			root = repositoryRoot(filepath.Dir(path))
		}
		if rel, err := relativeDir(dir, root); err == nil {
			opts.Root = rel
		}
	}
	var page bytes.Buffer
	if err := r.Render(&page, opts); err != nil {
		return err
	}
	if ruleRenderOutput == "" {
		_, err := cmd.OutOrStdout().Write(page.Bytes())
		return err
	}
	return os.WriteFile(ruleRenderOutput, page.Bytes(), 0o644)
}

// errInvalidRules is returned when the file has problems, once they are printed.
var errInvalidRules = errors.New("the rules have problems")

// loadAndCheck reads the file of rules and checks it, printing each problem prefixed by the path
// of the file. The files of the teacher are checked in root, or in the Git repository of the file
// when root is empty.
func loadAndCheck(path, root string, stderr io.Writer) (*rubric.Rubric, error) {
	if root == "" {
		root = repositoryRoot(filepath.Dir(path))
	}
	return loadRules(path, rubric.CheckOptions{Root: root, MaxWidth: maxRuleLineWidth}, stderr)
}

// loadRules reads the file of rules and checks it with the options, printing each problem
// prefixed by the path of the file.
func loadRules(path string, opts rubric.CheckOptions, stderr io.Writer) (*rubric.Rubric, error) {
	r, err := rubric.Load(path)
	var problems rubric.Problems
	if errors.As(err, &problems) {
		printProblems(stderr, path, problems)
		return nil, errInvalidRules
	}
	if err != nil {
		return nil, err
	}
	if problems := r.Check(opts); len(problems) > 0 {
		printProblems(stderr, path, problems)
		return nil, errInvalidRules
	}
	return r, nil
}

func printProblems(w io.Writer, path string, problems []rubric.Problem) {
	for _, problem := range problems {
		fmt.Fprintf(w, "%s:%s\n", path, problem)
	}
}

// relativeDir returns the path from the directory to the root, both made absolute first.
func relativeDir(dir, root string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// On macOS, /tmp is a link to /private/tmp, and git prints the target.
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	rel, err := filepath.Rel(absDir, absRoot)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// repositoryRoot returns the top of the Git repository of the directory, or the directory itself
// outside of a repository.
func repositoryRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}
	return strings.TrimSpace(string(out))
}
