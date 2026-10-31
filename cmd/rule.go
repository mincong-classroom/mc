package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var ruleLab string

var ruleCmd = &cobra.Command{
	Use:   "rule [file]",
	Short: "List, check and render the evaluation rules of a YAML file",
	Long: `List the evaluation rules of a YAML file, e.g. evaluation-rules.yaml: one line per rule, with
its exercise, its coefficient and its name.

Without a file, list the rules of 2025 written in the Go code of mc, which are deprecated.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRule,
	Example: `  mc rule evaluation-rules.yaml -l L1
  mc rule check evaluation-rules.yaml
  mc rule render evaluation-rules.yaml -o evaluation-rules.md`,
}

var ruleCheckRoot string

var ruleCheckCmd = &cobra.Command{
	Use:   "check <file>",
	Short: "Check the YAML of the evaluation rules, and exit with 1 on a problem",
	Long: `Check the YAML of the evaluation rules: the keys, the IDs of the labs and the rules, the
exercises, the coefficients (by steps of 0.05, those of the mandatory rules of a lab add up to 1),
the penalties, the levels of analysis, the kinds of evidence, the dependencies, the deadlines, the
paths, and the width of the lines (120 characters at most).

Print one problem per line, with the line of the file, and exit with 1 if there is any.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runRuleCheck,
	SilenceUsage: true,
}

var (
	ruleRenderOutput string
	ruleRenderRoot   string
)

var ruleRenderCmd = &cobra.Command{
	Use:   "render <file>",
	Short: "Render the evaluation rules of the YAML as a Markdown page",
	Long: `Check the YAML of the evaluation rules, then render it as a Markdown page: the labs, how the
score works, the evidence, the observations, then each lab with the table of its rules and each
rule. The texts are wrapped at 96 characters.`,
	Args:         cobra.ExactArgs(1),
	RunE:         runRuleRender,
	SilenceUsage: true,
}

// maxRuleLineWidth is the width of a line of the YAML, so that it reads without scrolling in a
// browser.
const maxRuleLineWidth = 120

const rootFlagUsage = "The repository of the rules, against which the paths of the teacher are checked (default: the Git repository of the file)"

func init() {
	ruleCmd.Flags().StringVarP(&ruleLab, "lab", "l", "", "List the rules of one lab session only, e.g. L1 or 1")
	ruleCheckCmd.Flags().StringVar(&ruleCheckRoot, "root", "", rootFlagUsage)
	ruleRenderCmd.Flags().StringVarP(&ruleRenderOutput, "output", "o", "", "The Markdown file to write (default: the standard output)")
	ruleRenderCmd.Flags().StringVar(&ruleRenderRoot, "root", "", rootFlagUsage)
	ruleCmd.AddCommand(ruleCheckCmd, ruleRenderCmd)
}

func runRule(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Deprecated: these are the rules of 2025. Pass the YAML of the rules, e.g. mc rule evaluation-rules.yaml.")
		printDeprecatedRules()
		return nil
	}
	r, err := rubric.Load(args[0])
	if err != nil {
		return err
	}
	labID := rubric.LabID(ruleLab)
	if labID != "" {
		if _, ok := r.Lab(labID); !ok {
			return fmt.Errorf("no lab %q in %s", ruleLab, args[0])
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

func printDeprecatedRules() {
	grader, err := rules.NewGrader()
	if err != nil {
		log.Fatalf("Failed to create grader: %v", err)
	}
	labID := rubric.LabID(ruleLab)
	found := false
	for _, spec := range grader.ListRuleSpecs() {
		if labID != "" && spec.LabId != labID {
			continue
		}
		found = true
		fmt.Println(spec.Representation())
		fmt.Println()
	}
	if !found {
		log.Fatalf("No rule for the lab %q: use L1 to L5", ruleLab)
	}
}

func runRuleCheck(cmd *cobra.Command, args []string) error {
	_, err := loadAndCheck(args[0], ruleCheckRoot, cmd.ErrOrStderr())
	return err
}

func runRuleRender(cmd *cobra.Command, args []string) error {
	path := args[0]
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
// of the file.
func loadAndCheck(path, root string, stderr io.Writer) (*rubric.Rubric, error) {
	r, err := rubric.Load(path)
	var problems rubric.Problems
	if errors.As(err, &problems) {
		printProblems(stderr, path, problems)
		return nil, errInvalidRules
	}
	if err != nil {
		return nil, err
	}
	if root == "" {
		root = repositoryRoot(filepath.Dir(path))
	}
	if problems := r.Check(rubric.CheckOptions{Root: root, MaxWidth: maxRuleLineWidth}); len(problems) > 0 {
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
