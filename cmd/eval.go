package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Check the evaluations of a lab, one YAML file per team",
	Long: `Check the evaluations of a lab, as the evaluator writes them: one YAML file per team, so that
several evaluators can work at once, in ~/.mc/{year}/evaluations/lab-{n}/{team}.yaml:

  team: teal
  commit: 0a1b2c3                    # the commit evaluated, the one of the facts of mc evidence
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
	Example: `  mc eval check -l L2
  mc eval check -l 2 -t teal`,
}

var (
	evalCheckRules string
	evalCheckLab   string
	evalCheckTeams []string
	evalCheckDir   string
)

var evalCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check the evaluations of a lab against the rules and the facts of mc evidence, and exit with 1 on a problem",
	Long: `Check the evaluations of a lab against the evaluation rules of the year, from the key rules of
~/.mc/settings.yaml, or the file of --rules-file:

  - the keys: an unknown key is a problem, and so is a key twice
  - the team, the name of the file
  - the commit, the one of the facts that mc evidence gathered for the team, in the report of the
    evidence on its branch teacher, such as ~/.mc/{year}/teacher/k8s-teal/docs/lab-2.evidence.md.
    A short hash matches the full one
  - the action items, if any, one of "Run it yourself", "Talk with them", "Less attention" and
    "Late work to decide"
  - the rules: each mandatory rule of the lab, an optional one if evaluated, and no other rule
  - for each rule, the completeness from 0 to 1 by steps of 0.1, what the team did well and what
    it can improve, the analysis among the analysis levels, and the writing style
  - the penalty of a rule, if any: its kind, a penalty of every rule such as secret, "rule" for
    the penalty of the rule itself, or "late"; the share of the score that the rules give for it,
    from the min to the max of the late penalty for late work; and why

Print one problem per line, with the file and its line, and exit with 1 if there is any.`,
	Args:         cobra.NoArgs,
	RunE:         runEvalCheck,
	SilenceUsage: true,
}

func init() {
	evalCheckCmd.Flags().StringVar(&evalCheckRules, "rules-file", "", rulesFileUsage)
	evalCheckCmd.Flags().StringVarP(&evalCheckLab, "lab", "l", "", "The lab, e.g. L2 or 2")
	evalCheckCmd.Flags().StringArrayVarP(&evalCheckTeams, "team", "t", nil, "A team, repeatable (default: every team with a file)")
	evalCheckCmd.Flags().StringVar(&evalCheckDir, "dir", "", "The directory of the evaluations, one directory per lab such as lab-2 (default: ~/.mc/{year}/evaluations)")
	_ = evalCheckCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	evalCmd.AddCommand(evalCheckCmd)
	RootCmd.AddCommand(evalCmd)
}

func runEvalCheck(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(evalCheckRules)
	if err != nil {
		return err
	}
	// The content of the rules only: the files of the teacher don't change an evaluation.
	r, err := loadRules(path, rubric.CheckOptions{}, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	lab, ok := r.Lab(rubric.LabID(evalCheckLab))
	if !ok {
		return fmt.Errorf("no lab %q in %s", evalCheckLab, path)
	}
	dir := common.ExpandHome(evalCheckDir)
	if dir == "" {
		dir = filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "evaluations")
	}
	labDir := evaluation.LabDir(dir, lab)
	names := slices.Clone(evalCheckTeams)
	if len(names) == 0 {
		if names, err = evaluation.Teams(labDir); err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("no evaluation of %s in %s: one YAML file per team, such as %s", lab.ID, labDir, evaluation.Path(labDir, "teal"))
		}
	}
	slices.Sort(names)
	names = slices.Compact(names)
	teams, err := common.ListTeams()
	if err != nil {
		return err
	}

	c := evalChecker{rubric: r, lab: lab, teams: teams, teacherDir: teacherDir()}
	count, failed := 0, 0
	for _, name := range names {
		file := evaluation.Path(labDir, name)
		problems, err := c.check(file, name)
		if err != nil {
			return err
		}
		printEvaluationProblems(cmd.ErrOrStderr(), file, problems)
		if len(problems) > 0 {
			count += len(problems)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%s in %d of %s", plural(count, "problem"), failed, plural(len(names), "file"))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s of %s checked in %s: no problem\n", plural(len(names), "file"), lab.ID, labDir)
	return nil
}

// evalChecker checks the files of the evaluations of a lab.
type evalChecker struct {
	rubric     *rubric.Rubric
	lab        rubric.Lab
	teams      []common.Team // The teams of the registry.
	teacherDir string        // The worktrees of the branch teacher, with the facts of mc evidence.
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

	// The worktree of a team is named after its repository, as mc evidence names it.
	team := common.Team{Name: name}
	if i := slices.IndexFunc(c.teams, func(t common.Team) bool { return t.Name == name }); i >= 0 {
		team = c.teams[i]
	} else {
		problems = append(problems, evaluation.Problem{Message: fmt.Sprintf("the team %q is not in the registry %s", name, common.TeamRegistryPath())})
	}
	facts, ok, err := evidence.ReadFacts(evidence.ReportPath(filepath.Join(c.teacherDir, team.GetRepoName()), c.lab))
	if err != nil {
		return nil, err
	}
	opts := evaluation.CheckOptions{Rubric: c.rubric, Lab: c.lab.ID, Team: name}
	if ok {
		opts.Commit = facts.Commit
	}
	return append(problems, f.Check(opts)...), nil
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
// evidence writes its reports.
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
