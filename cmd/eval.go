package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate the labs, then list, check and sync their evaluations",
	Long: `Evaluate a lab with mc eval run: the facts of each team, then its report and its grades, which AI
agents write, each step when its inputs changed. mc eval gather-facts gathers the facts only. Then
list, check and sync the evaluations of the labs, as the evaluator writes them: one YAML file per
team and lab, so that several evaluators can work at once, in
~/.mc/{year}/evaluations/lab-{n}/{team}.yaml:

  team: teal
  commit: 0a1b2c3                    # the commit evaluated, the one of the facts
  time_spent: Session only, done at 11:21
  action_items: Run it yourself      # or Talk with them, Less attention, Late work to decide
  rules:
    L2_POD:
      completeness: 0.8              # from 0 to 1, by steps of 0.1
      done_well: The Pod runs the image of Lab 1.
      to_improve: The report doesn't show the logs of the Pod.
      to_check: ""                   # what the teacher should check, if anything
      analysis: short                # a key of the analysis levels of the rules
      writing_style: own words
      penalty: null                  # or {kind: secret, remove: 0.3, why: ...}`,
	Example: `  mc eval run -l L2
  mc eval run -l 2 -t teal --skip report
  mc eval run -l 2 --check-only
  mc eval gather-facts -l L2
  mc eval ls -l L2`,
}

var (
	evalRules string
	evalDir   string
)

var evalLsLab string

var evalLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List the evaluations of the labs",
	Long: `List the evaluations of the labs, or of one lab with --lab: one line per file, with its lab,
its team, the commit evaluated, the mandatory rules evaluated out of those of the lab (and the
optional ones after a +), the rules that the teacher should check, the penalties, and the action
items.

A file that can't be read is listed with dashes: "mc eval run" gives its problems.`,
	Args:         cobra.NoArgs,
	RunE:         runEvalLs,
	SilenceUsage: true,
}

func init() {
	evalCmd.PersistentFlags().StringVar(&evalRules, "rules-file", "", rulesFileUsage)
	evalCmd.PersistentFlags().StringVar(&evalDir, "dir", "", "The directory of the evaluations, one directory per lab such as lab-2 (default: ~/.mc/{year}/evaluations)")
	evalLsCmd.Flags().StringVarP(&evalLsLab, "lab", "l", "", "List the evaluations of one lab only, e.g. L2 or 2")
	evalCmd.AddCommand(evalLsCmd)
	RootCmd.AddCommand(evalCmd)
}

// loadEvalRules reads the rules of the year, or the file of --rules-file, and returns them with
// their path.
func loadEvalRules(w io.Writer) (*rubric.Rubric, string, error) {
	path, err := common.RulesPath(evalRules)
	if err != nil {
		return nil, "", err
	}
	// The content of the rules only: the files of the teacher don't change an evaluation.
	r, err := loadRules(path, rubric.CheckOptions{}, w)
	return r, path, err
}

// evaluationsDir returns the directory of the evaluations: the flag --dir, or the one of the year.
func evaluationsDir() string {
	if evalDir != "" {
		return common.ExpandHome(evalDir)
	}
	return filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "evaluations")
}

func runEvalLs(cmd *cobra.Command, _ []string) error {
	r, path, err := loadEvalRules(cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	labs := r.Labs
	if evalLsLab != "" {
		lab, ok := r.Lab(rubric.LabID(evalLsLab))
		if !ok {
			return fmt.Errorf("no lab %q in %s", evalLsLab, path)
		}
		labs = []rubric.Lab{lab}
	}
	dir := evaluationsDir()

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	header := false
	for _, lab := range labs {
		labDir := evaluation.LabDir(dir, lab)
		names, err := evaluation.Teams(labDir)
		if err != nil {
			return err
		}
		for _, name := range names {
			if !header {
				fmt.Fprintln(w, "LAB\tTEAM\tCOMMIT\tRULES\tTO CHECK\tPENALTIES\tACTION ITEMS")
				header = true
			}
			file := evaluation.Path(labDir, name)
			f, err := evaluation.Load(file)
			var problems evaluation.Problems
			switch {
			case errors.As(err, &problems):
				fmt.Fprintf(w, "%s\t%s\t-\t-\t-\t-\t-\n", lab.ID, name)
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s, see mc eval run -l %s -t %s\n", file, plural(len(problems), "problem"), lab.ID, name)
				continue
			case err != nil:
				return err
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n", lab.ID, name, shortCommit(f.Commit), evaluatedRules(f, r.LabRules(lab.ID)),
				countRules(f.Rules, func(rule evaluation.Rule) bool { return rule.ToCheck != "" }),
				countRules(f.Rules, func(rule evaluation.Rule) bool { return rule.Penalty != nil }),
				orDash(f.ActionItems))
		}
	}
	if !header {
		where := dir
		if evalLsLab != "" {
			where = evaluation.LabDir(dir, labs[0])
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No evaluation in %s\n", where)
		return nil
	}
	return w.Flush()
}

// evaluatedRules returns the mandatory rules of the lab that the file evaluates, out of all of
// them, then the optional ones after a +: "2/2", or "2/2 +1".
func evaluatedRules(f *evaluation.File, rules []rubric.Rule) string {
	evaluated := map[string]bool{}
	for _, rule := range f.Rules {
		evaluated[rule.ID] = true
	}
	mandatory, done, bonus := 0, 0, 0
	for _, rule := range rules {
		switch {
		case !rule.Optional():
			mandatory++
			if evaluated[rule.ID] {
				done++
			}
		case evaluated[rule.ID]:
			bonus++
		}
	}
	s := fmt.Sprintf("%d/%d", done, mandatory)
	if bonus > 0 {
		s += fmt.Sprintf(" +%d", bonus)
	}
	return s
}

// countRules returns the number of rules for which keep is true.
func countRules(rules []evaluation.Rule, keep func(evaluation.Rule) bool) int {
	n := 0
	for _, rule := range rules {
		if keep(rule) {
			n++
		}
	}
	return n
}

// shortCommit returns the first 7 characters of a commit, or a dash without one.
func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return orDash(commit)
}

// orDash returns s, or a dash when it is empty, so that a column of a table is never empty.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// labTeams returns the teams, or else those of the files of the lab, sorted and without a duplicate.
func labTeams(labDir string, lab rubric.Lab, teams []string) ([]string, error) {
	names := slices.Clone(teams)
	if len(names) == 0 {
		var err error
		if names, err = evaluation.Teams(labDir); err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("no evaluation of %s in %s: one YAML file per team, such as %s", lab.ID, labDir, evaluation.Path(labDir, "teal"))
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// checkEvaluations checks the files of the teams in the lab, prints their problems to w, and
// returns an error if there is any.
func checkEvaluations(w io.Writer, r *rubric.Rubric, lab rubric.Lab, labDir string, names []string) error {
	problems, err := checkTeams(w, r, lab, labDir, names)
	if err != nil {
		return err
	}
	return problemsError(problems, len(names))
}

// checkTeams checks the files of the teams in the lab, prints their problems to w, and returns the
// number of problems of each team that has any.
func checkTeams(w io.Writer, r *rubric.Rubric, lab rubric.Lab, labDir string, names []string) (map[string]int, error) {
	teams, err := common.ListTeams()
	if err != nil {
		return nil, err
	}
	c := evalChecker{rubric: r, lab: lab, teams: teams, teacherDir: teacherDir()}
	problems := map[string]int{}
	for _, name := range names {
		file := evaluation.Path(labDir, name)
		found, err := c.check(file, name)
		if err != nil {
			return nil, err
		}
		printEvaluationProblems(w, file, found)
		if len(found) > 0 {
			problems[name] = len(found)
		}
	}
	return problems, nil
}

// problemsError returns the error of the problems of the files, by team, or nil without any: "3
// problems in 2 of 5 files".
func problemsError(problems map[string]int, files int) error {
	if len(problems) == 0 {
		return nil
	}
	total := 0
	for _, n := range problems {
		total += n
	}
	return fmt.Errorf("%s in %d of %s", plural(total, "problem"), len(problems), plural(files, "file"))
}

// evalChecker checks the files of the evaluations of a lab.
type evalChecker struct {
	rubric     *rubric.Rubric
	lab        rubric.Lab
	teams      []common.Team // The teams of the registry.
	teacherDir string        // The worktrees of the branch teacher, with the facts.
}

// check returns the problems of the file of the team. Its error is one of the facts, which is not
// a problem of the file.
func (c evalChecker) check(file, name string) ([]evaluation.Problem, error) {
	f, err := evaluation.Load(file)
	var problems evaluation.Problems
	switch {
	case errors.As(err, &problems):
		return problems, nil
	case errors.Is(err, fs.ErrNotExist):
		return []evaluation.Problem{{Message: "no such file: the evaluation of the team " + name}}, nil
	case err != nil:
		return nil, err
	}

	team, found := teamOf(c.teams, name)
	if !found {
		problems = append(problems, evaluation.Problem{Message: fmt.Sprintf("the team %q is not in the registry %s", name, common.TeamRegistryPath())})
	}
	facts, ok, err := evidence.ReadFacts(factsOf(c.teacherDir, team, c.lab))
	if err != nil {
		return nil, err
	}
	opts := evaluation.CheckOptions{Rubric: c.rubric, Lab: c.lab.ID, Team: name}
	if ok {
		opts.Commit = facts.Commit
	}
	return append(problems, f.Check(opts)...), nil
}

// teamOf returns the team of the registry with the name, or a team of that name alone when the
// registry doesn't have it.
func teamOf(teams []common.Team, name string) (common.Team, bool) {
	if i := slices.IndexFunc(teams, func(t common.Team) bool { return t.Name == name }); i >= 0 {
		return teams[i], true
	}
	return common.Team{Name: name}, false
}

// factsOf returns the file of the facts of the lab for the team, in its worktree of the branch
// teacher, which is named after its repository, as mc eval gather-facts names it.
func factsOf(teacherDir string, team common.Team, lab rubric.Lab) string {
	return evidence.FactsPath(filepath.Join(teacherDir, team.GetRepoName()), lab)
}

// printEvaluationProblems prints each problem prefixed by the path of the file, and its line if
// known: file:12: L2_POD: message.
func printEvaluationProblems(w io.Writer, path string, problems []evaluation.Problem) {
	for _, problem := range problems {
		separator := ":"
		if problem.Line == 0 {
			separator = ": "
		}
		fmt.Fprintf(w, "%s%s%s\n", path, separator, problem)
	}
}

// teacherDir returns the directory of the worktrees of the branch teacher, one per team, where mc
// eval gather-facts writes the facts.
func teacherDir() string {
	return filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "teacher")
}

// plural returns the count and the noun, with an s unless the count is 1.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(count) + " " + noun + "s"
}
