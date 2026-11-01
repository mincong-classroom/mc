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
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "List and check the evaluations of the labs, one YAML file per team and lab",
	Long: `List and check the evaluations of the labs, as the evaluator writes them: one YAML file per
team and lab, so that several evaluators can work at once, in
~/.mc/{year}/evaluations/lab-{n}/{team}.yaml:

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
	Example: `  mc eval ls
  mc eval ls -l L2
  mc eval run -l L2
  mc eval run -l 2 -t teal
  mc eval run -l 2 --check-only`,
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

var (
	evalRunLab       string
	evalRunTeams     []string
	evalRunCheckOnly bool
)

var evalRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Evaluate a lab with the agent of the settings, then check its files, and exit with 1 on a problem",
	Long: `Evaluate a lab with the AI agent of the key agents.eval_run of ~/.mc/settings.yaml, then check
the files that it wrote. The agent is a command line, run with sh in the Git repository of the
rules, where {lab} is the lab and {teams} the teams of --team, or all:

  agents:
    eval_run: claude -p "/evaluate-lab {lab} {teams}"

The agent does the work, such as gathering the evidence with mc evidence, and writes one file per
team. It runs with the variable MC_EVAL_AGENT set, so that it can't start another agent: it checks
its files with mc eval run --check-only.

The checks compare the files with the evaluation rules of the year, from the key rules of
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
	RunE:         runEvalRun,
	SilenceUsage: true,
}

func init() {
	evalCmd.PersistentFlags().StringVar(&evalRules, "rules-file", "", rulesFileUsage)
	evalCmd.PersistentFlags().StringVar(&evalDir, "dir", "", "The directory of the evaluations, one directory per lab such as lab-2 (default: ~/.mc/{year}/evaluations)")
	evalLsCmd.Flags().StringVarP(&evalLsLab, "lab", "l", "", "List the evaluations of one lab only, e.g. L2 or 2")
	evalRunCmd.Flags().StringVarP(&evalRunLab, "lab", "l", "", "The lab, e.g. L2 or 2")
	evalRunCmd.Flags().StringArrayVarP(&evalRunTeams, "team", "t", nil, "A team, repeatable (default: every team, all for the agent)")
	evalRunCmd.Flags().BoolVar(&evalRunCheckOnly, "check-only", false, "Check the files only, without the agent, e.g. from the agent itself")
	_ = evalRunCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	evalCmd.AddCommand(evalLsCmd, evalRunCmd)
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
				count(f.Rules, func(rule evaluation.Rule) bool { return rule.ToCheck != "" }),
				count(f.Rules, func(rule evaluation.Rule) bool { return rule.Penalty != nil }),
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

// count returns the number of rules for which keep is true.
func count(rules []evaluation.Rule, keep func(evaluation.Rule) bool) int {
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

func runEvalRun(cmd *cobra.Command, _ []string) error {
	r, path, err := loadEvalRules(cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	lab, ok := r.Lab(rubric.LabID(evalRunLab))
	if !ok {
		return fmt.Errorf("no lab %q in %s", evalRunLab, path)
	}
	if !evalRunCheckOnly {
		if err := runEvalAgent(cmd, lab, evalRunTeams, path); err != nil {
			return err
		}
	}
	labDir := evaluation.LabDir(evaluationsDir(), lab)
	names := slices.Clone(evalRunTeams)
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
	total, failed := 0, 0
	for _, name := range names {
		file := evaluation.Path(labDir, name)
		problems, err := c.check(file, name)
		if err != nil {
			return err
		}
		printEvaluationProblems(cmd.ErrOrStderr(), file, problems)
		if len(problems) > 0 {
			total += len(problems)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%s in %d of %s", plural(total, "problem"), failed, plural(len(names), "file"))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s of %s checked in %s: no problem\n", plural(len(names), "file"), lab.ID, labDir)
	return nil
}

// agentEnv is set for the agent of mc eval run, so that it can't start another agent.
const agentEnv = "MC_EVAL_AGENT"

// teamNamePattern matches the name of a team, which goes into the command line of the agent as it
// is.
var teamNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// runEvalAgent runs the agent of the settings that evaluates the lab for the teams, or for all of
// them, in the Git repository of the rules.
func runEvalAgent(cmd *cobra.Command, lab rubric.Lab, teams []string, rulesPath string) error {
	if os.Getenv(agentEnv) != "" {
		return fmt.Errorf("an agent of mc eval run can't start another one (%s is set): run mc eval run --check-only", agentEnv)
	}
	s, err := common.CurrentSettings()
	if err != nil {
		return err
	}
	if s.Agents.EvalRun == "" {
		return fmt.Errorf(`no agent to evaluate the lab: set agents.eval_run in %s, such as claude -p "/evaluate-lab {lab} {teams}", or pass --check-only`, common.SettingsPath())
	}
	for _, team := range teams {
		if !teamNamePattern.MatchString(team) {
			return fmt.Errorf("%q is not the name of a team: lowercase letters, digits and dashes", team)
		}
	}
	arg := "all"
	if len(teams) > 0 {
		arg = strings.Join(teams, " ")
	}
	line := strings.NewReplacer("{lab}", lab.ID, "{teams}", arg).Replace(s.Agents.EvalRun)
	dir := repositoryRoot(filepath.Dir(rulesPath))
	fmt.Fprintf(cmd.ErrOrStderr(), "Running the agent in %s: %s\n", dir, line)
	agent := exec.Command("sh", "-c", line)
	agent.Dir = dir
	agent.Env = append(os.Environ(), agentEnv+"=1")
	agent.Stdout = cmd.OutOrStdout()
	agent.Stderr = cmd.ErrOrStderr()
	if err := agent.Run(); err != nil {
		return fmt.Errorf("the agent failed, so its files are not checked: %w", err)
	}
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
