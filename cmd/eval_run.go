package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evaluation"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/rubric"
)

var (
	evalRunLab       string
	evalRunTeams     []string
	evalRunSkip      []string
	evalRunForce     []string
	evalRunLate      bool
	evalRunTemplate  string
	evalRunCheckOnly bool
)

var evalRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the steps of the evaluation of a lab, each one when its inputs changed, then check the evaluations",
	Long: `Run the steps of the evaluation of a lab for the teams, so that the teacher has the facts, a report
and a draft of the grades of each team:

  1. The facts, as mc eval gather-facts gathers them, for every team of the registry, or those of
     --team.
  2. The report: the agent of agents.report writes the report of each team from its facts, such as
     docs/lab-2.report.md on its branch teacher.
  3. The grades: the agent of agents.grade writes the evaluation of each team, one YAML file per
     team, in ~/.mc/{year}/evaluations/lab-{n}/{team}.yaml.
  4. The checks of the evaluations, never skipped.

An agent is a command line of ~/.mc/settings.yaml, run with sh in the Git repository of the rules,
where {lab} is the lab, {teams} the teams whose step runs, separated by spaces, and {late} is
"late" with --late, empty otherwise. Its inputs are the files and the directories that it reads,
such as its skill, relative to that repository:

  agents:
    report:
      run: claude -p "/report-lab {lab} {teams}"
      inputs: [.claude/skills/report-lab]
    grade:
      run: claude -p "/grade-lab {lab} {teams}"
      inputs: [.claude/skills/grade-lab]

A step runs for a team when one of its inputs changed since it last succeeded, or when its file is
missing. mc records the digests of the inputs in ~/.mc/{year}/evaluations/lab-{n}/.steps/{team}.json:

  - the facts: as mc eval gather-facts, they are up to date while the report has the commit
    evaluated, the commit of mc and the images of now
  - the report: the front matter of the facts without the commit of mc, and the inputs of its
    agent. A new version of mc that gathers the same facts writes no new report
  - the grades: the same, the report, and the rules of the lab without their coefficients, which
    change no completeness

The report succeeds for a team when its agent wrote it at the commit of the facts, and the grades
when the evaluation passes the checks. A team whose step fails stops there, and the others go on:
the command exits with 1 at the end. A team stops at the facts too when Docker Hub didn't give the
images of a rule.

The agents run with the variable MC_EVAL_AGENT set, so that they can't start another agent: an
agent checks its files with mc eval run --check-only.

The checks compare the files with the evaluation rules of the year, from the key rules of
~/.mc/settings.yaml, or the file of --rules-file:

  - the keys: an unknown key is a problem, and so is a key twice
  - the team, the name of the file
  - the commit, the one of the facts that mc eval gather-facts gathered for the team, on its branch
    teacher, such as ~/.mc/{year}/teacher/k8s-teal/docs/lab-2.facts.md. A short hash matches the
    full one
  - the action items, if any, one of "Run it yourself", "Talk with them", "Less attention" and
    "Late work to decide"
  - the rules: each mandatory rule of the lab, an optional one if evaluated, and no other rule
  - for each rule, the completeness from 0 to 1 by steps of 0.1, what the team did well and what
    it can improve, the analysis among the analysis levels, and the writing style
  - the penalty of a rule, if any: its kind, a penalty of every rule such as secret, "rule" for
    the penalty of the rule itself, or "late"; the share of the score that the rules give for it,
    from the min to the max of the late penalty for late work; and why

Print one problem per line, with the file and its line, and exit with 1 if there is any.`,
	Example: `  mc eval run -l L2
  mc eval run -l 2 -t teal -t coral
  mc eval run -l 2 --skip report --force grade
  mc eval run -l 1 --late -t teal --template ~/github/my-org/assignment
  mc eval run -l 2 --check-only`,
	Args:         cobra.NoArgs,
	RunE:         runEvalRun,
	SilenceUsage: true,
}

func init() {
	evalRunCmd.Flags().StringVarP(&evalRunLab, "lab", "l", "", "The lab, e.g. L2 or 2")
	evalRunCmd.Flags().StringArrayVarP(&evalRunTeams, "team", "t", nil, "A team, repeatable (default: every team of the registry, or every file with --check-only)")
	evalRunCmd.Flags().StringArrayVar(&evalRunSkip, "skip", nil, "A step to skip, repeatable: facts, report or grade")
	evalRunCmd.Flags().StringArrayVar(&evalRunForce, "force", nil, "A step to run even when its inputs didn't change, repeatable: facts, report or grade")
	evalRunCmd.Flags().BoolVar(&evalRunLate, "late", false, "Gather the facts of origin/main, for a late pass, and give {late} to the agents")
	evalRunCmd.Flags().StringVar(&evalRunTemplate, "template", "", "The clone of the template of the team repositories, for the facts (default: the first commit of each repository)")
	evalRunCmd.Flags().BoolVar(&evalRunCheckOnly, "check-only", false, "Check the files only, without the steps, e.g. from an agent")
	_ = evalRunCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	evalCmd.AddCommand(evalRunCmd)
}

// The steps of mc eval run, as the flags --skip and --force name them.
const (
	stepFacts  = "facts"
	stepReport = "report"
	stepGrade  = "grade"
)

// evalSteps are the steps, in their order.
var evalSteps = []string{stepFacts, stepReport, stepGrade}

// stepNames are the names of the steps in a sentence.
var stepNames = map[string]string{stepFacts: "facts", stepReport: "report", stepGrade: "grades"}

// agentEnv is set for the agents of mc eval run, so that they can't start another agent.
const agentEnv = "MC_EVAL_AGENT"

// teamNamePattern matches the name of a team, which goes into the command line of an agent as it
// is.
var teamNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func runEvalRun(cmd *cobra.Command, _ []string) error {
	r, path, err := loadEvalRules(cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	lab, ok := r.Lab(rubric.LabID(evalRunLab))
	if !ok {
		return fmt.Errorf("no lab %q in %s", evalRunLab, path)
	}
	labDir := evaluation.LabDir(evaluationsDir(), lab)
	if evalRunCheckOnly {
		if len(evalRunSkip) > 0 || len(evalRunForce) > 0 || evalRunLate || evalRunTemplate != "" {
			return errors.New("--check-only runs no step: drop --skip, --force, --late and --template")
		}
		names, err := labTeams(labDir, lab, evalRunTeams)
		if err != nil {
			return err
		}
		if err := checkEvaluations(cmd.ErrOrStderr(), r, lab, labDir, names); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s of %s checked in %s: no problem\n", plural(len(names), "file"), lab.ID, labDir)
		return nil
	}

	if os.Getenv(agentEnv) != "" {
		return fmt.Errorf("an agent of mc eval run can't start another one (%s is set): run mc eval run --check-only", agentEnv)
	}
	skip, err := parseSteps("--skip", evalRunSkip)
	if err != nil {
		return err
	}
	force, err := parseSteps("--force", evalRunForce)
	if err != nil {
		return err
	}
	for _, step := range evalSteps {
		if skip[step] && force[step] {
			return fmt.Errorf("--skip %s and --force %s are exclusive", step, step)
		}
	}
	teams, err := common.ListTeams()
	if err != nil {
		return err
	}
	if len(evalRunTeams) > 0 {
		if teams, err = common.FilterTeams(teams, evalRunTeams); err != nil {
			return err
		}
	}
	for _, team := range teams {
		if !teamNamePattern.MatchString(team.Name) {
			return fmt.Errorf("%q is not the name of a team: lowercase letters, digits and dashes", team.Name)
		}
	}
	settings, err := common.CurrentSettings()
	if err != nil {
		return err
	}

	run := &evalRun{
		out:       cmd.OutOrStdout(),
		errOut:    cmd.ErrOrStderr(),
		rubric:    r,
		lab:       lab,
		labDir:    labDir,
		repoDir:   repositoryRoot(filepath.Dir(path)),
		worktrees: teacherDir(),
		teams:     teams,
		skip:      skip,
		force:     force,
		late:      evalRunLate,
		template:  evalRunTemplate,
	}
	if err := run.prepare(settings.Agents); err != nil {
		return err
	}
	return run.run()
}

// parseSteps returns the steps of a flag, such as --skip report, as a set.
func parseSteps(flag string, values []string) (map[string]bool, error) {
	steps := map[string]bool{}
	for _, value := range values {
		if !slices.Contains(evalSteps, value) {
			return nil, fmt.Errorf("%s %q is not a step: %s", flag, value, strings.Join(evalSteps, ", "))
		}
		steps[value] = true
	}
	return steps, nil
}

// evalRun is a run of the steps of the evaluation of a lab.
type evalRun struct {
	out, errOut io.Writer
	rubric      *rubric.Rubric
	lab         rubric.Lab
	labDir      string // The directory of the evaluations of the lab, and of the records of the steps.
	repoDir     string // The Git repository of the rules, where the agents run.
	worktrees   string // The directory of the worktrees of the branch teacher, one per team.
	teams       []common.Team
	skip, force map[string]bool
	late        bool
	template    string

	agents  map[string]common.Agent      // The agent of each step that runs, by step.
	inputs  map[string]map[string]string // The digests of the inputs of each agent in the settings, by step.
	rules   string                       // The digest of the rules of the lab.
	records map[string]*evaluation.Steps // The record of each team.
	facts   map[string]evidence.Facts    // The facts of each team that goes on.
	digests map[string]string            // The digest of the facts of each team that goes on.
	stopped map[string]string            // The step at which each team stopped.
}

// prepare reads the agents of the steps that run and the digests of their inputs, and the records
// of the teams, so that a mistake of the settings stops the run before any step.
func (e *evalRun) prepare(agents common.Agents) error {
	e.agents, e.inputs = map[string]common.Agent{}, map[string]map[string]string{}
	for _, step := range []struct {
		name    string
		agent   common.Agent
		example string
	}{
		{stepReport, agents.Report, `claude -p "/report-lab {lab} {teams}"`},
		{stepGrade, agents.Grade, `claude -p "/grade-lab {lab} {teams}"`},
	} {
		if e.skip[step.name] {
			continue
		}
		if step.agent.Run == "" {
			return fmt.Errorf("no agent for the %s: set agents.%s.run in %s, such as %s, or pass --skip %s",
				stepNames[step.name], step.name, common.SettingsPath(), step.example, step.name)
		}
		paths := make([]string, len(step.agent.Inputs))
		for i, input := range step.agent.Inputs {
			paths[i] = common.ExpandHome(input)
		}
		files, err := evaluation.DigestFiles(e.repoDir, paths)
		if err != nil {
			return fmt.Errorf("agents.%s.inputs of %s, in %s: %w", step.name, common.SettingsPath(), e.repoDir, err)
		}
		e.agents[step.name], e.inputs[step.name] = step.agent, files
	}
	var err error
	if e.rules, err = evaluation.DigestRules(e.rubric, e.lab.ID); err != nil {
		return err
	}
	e.records = map[string]*evaluation.Steps{}
	e.stopped = map[string]string{}
	for _, team := range e.teams {
		record, err := evaluation.ReadSteps(e.labDir, team.Name)
		if err != nil {
			return err
		}
		e.records[team.Name] = &record
	}
	return nil
}

func (e *evalRun) run() error {
	if e.skip[stepFacts] {
		fmt.Fprintln(e.out, "The facts: skipped (--skip facts)")
	} else {
		pass := factsPass{
			rubric:    e.rubric,
			lab:       e.lab.ID,
			dir:       e.worktrees,
			late:      e.late,
			template:  e.template,
			force:     e.force[stepFacts],
			prefix:    "facts: ",
			forceFlag: "--force facts",
		}
		failed, err := pass.gather(e.out, e.teams)
		if err != nil {
			return err
		}
		for _, team := range e.teams {
			if err, ok := failed[team.Name]; ok {
				e.stop(team.Name, stepFacts, err.Error())
			}
		}
	}
	if err := e.readFacts(); err != nil {
		return err
	}

	due, inputs, err := e.due(stepReport)
	if err != nil {
		return err
	}
	if len(due) > 0 {
		if err := e.runAgent(stepReport, due); err != nil {
			e.stopAll(due, stepReport, err)
		} else if err := e.recordReports(due, inputs); err != nil {
			return err
		}
	}

	due, inputs, err = e.due(stepGrade)
	if err != nil {
		return err
	}
	if len(due) > 0 {
		if err := e.runAgent(stepGrade, due); err != nil {
			e.stopAll(due, stepGrade, err)
			due = nil
		}
	}
	return e.check(due, inputs)
}

// readFacts reads the facts of each team that goes on, and stops a team without facts, or whose
// facts miss the images of a rule, as when Docker Hub couldn't be read.
func (e *evalRun) readFacts() error {
	e.facts, e.digests = map[string]evidence.Facts{}, map[string]string{}
	for _, team := range e.going() {
		file := factsOf(e.worktrees, team, e.lab)
		facts, ok, err := evidence.ReadFacts(file)
		if err != nil {
			e.stop(team.Name, stepFacts, err.Error())
			continue
		}
		if !ok {
			e.stop(team.Name, stepFacts, "no facts in "+file+": gather them, without --skip facts")
			continue
		}
		if rule, ok := e.ruleWithoutImages(facts); ok {
			e.stop(team.Name, stepFacts, fmt.Sprintf("no images of %s in %s, as when Docker Hub couldn't be read: gather the facts again", rule, file))
			continue
		}
		// The commit of mc is left out: a new version of mc that gathers the same facts runs no step again.
		digest, err := evaluation.Digest(struct {
			Commit string
			Rules  map[string]evidence.RuleFacts
		}{facts.Commit, facts.Rules})
		if err != nil {
			return err
		}
		e.facts[team.Name], e.digests[team.Name] = facts, digest
	}
	return nil
}

// ruleWithoutImages returns the first rule of the lab that has images, but whose facts have none:
// "none" is a fact, whereas empty means that Docker Hub couldn't be read.
func (e *evalRun) ruleWithoutImages(facts evidence.Facts) (string, bool) {
	for _, rule := range e.rubric.LabRules(e.lab.ID) {
		if len(rule.Images) > 0 && facts.Rules[rule.ID].Images == "" {
			return rule.ID, true
		}
	}
	return "", false
}

// due returns the teams for which the step runs, in the order of their names, and their inputs:
// those whose inputs changed since the step last succeeded, or without the file of the step. It
// prints one line per team.
func (e *evalRun) due(step string) ([]string, map[string]evaluation.Inputs, error) {
	if e.skip[step] {
		fmt.Fprintf(e.out, "The %s: skipped (--skip %s)\n", stepNames[step], step)
		return nil, nil, nil
	}
	var due []string
	inputs := map[string]evaluation.Inputs{}
	for _, team := range e.going() {
		now := evaluation.Inputs{Facts: e.digests[team.Name], Files: e.inputs[step]}
		output, noun := evidence.ReportPath(filepath.Join(e.worktrees, team.GetRepoName()), e.lab), "report"
		former := e.records[team.Name].Report
		if step == stepGrade {
			report, err := evaluation.DigestFile(output)
			if err != nil {
				return nil, nil, err
			}
			now.Report, now.Rules = report, e.rules
			output, noun = evaluation.Path(e.labDir, team.Name), "evaluation"
			former = e.records[team.Name].Grade
		}

		var reasons []string
		if e.force[step] {
			reasons = append(reasons, "--force "+step)
		}
		switch _, err := os.Stat(output); {
		case errors.Is(err, os.ErrNotExist):
			reasons = append(reasons, "no "+noun+" yet")
		case err != nil:
			return nil, nil, err
		case former == nil:
			reasons = append(reasons, "no record of a former run")
		default:
			reasons = append(reasons, evaluation.Changes(former.Inputs, now)...)
		}
		if len(reasons) == 0 {
			fmt.Fprintf(e.out, "[team: %s] %s: up to date\n", team.Name, step)
			continue
		}
		fmt.Fprintf(e.out, "[team: %s] %s: to run, %s\n", team.Name, step, strings.Join(reasons, ", "))
		due = append(due, team.Name)
		inputs[team.Name] = now
	}
	slices.Sort(due)
	return due, inputs, nil
}

// runAgent runs the agent of the step for the teams, with sh in the Git repository of the rules.
func (e *evalRun) runAgent(step string, teams []string) error {
	late := ""
	if e.late {
		late = "late"
	}
	line := strings.NewReplacer("{lab}", e.lab.ID, "{teams}", strings.Join(teams, " "), "{late}", late).Replace(e.agents[step].Run)
	fmt.Fprintf(e.errOut, "Running the agent of agents.%s in %s: %s\n", step, e.repoDir, line)
	agent := exec.Command("sh", "-c", line)
	agent.Dir = e.repoDir
	agent.Env = append(os.Environ(), agentEnv+"=1")
	agent.Stdout = e.out
	agent.Stderr = e.errOut
	if err := agent.Run(); err != nil {
		return fmt.Errorf("the agent failed: %w", err)
	}
	return nil
}

// recordReports records the report of each team whose agent wrote it at the commit of the facts,
// and stops the others.
func (e *evalRun) recordReports(teams []string, inputs map[string]evaluation.Inputs) error {
	for _, name := range teams {
		team, _ := teamOf(e.teams, name)
		file := evidence.ReportPath(filepath.Join(e.worktrees, team.GetRepoName()), e.lab)
		report, ok, err := evidence.ReadFacts(file) // The report copies the front matter of the facts.
		if err != nil {
			e.stop(name, stepReport, err.Error())
			continue
		}
		commit := e.facts[name].Commit
		if !ok || !sameCommit(report.Commit, commit) {
			e.stop(name, stepReport, fmt.Sprintf("no report at the commit of the facts, %s, in %s", commit[:min(len(commit), 7)], file))
			continue
		}
		if err := e.record(name, stepReport, inputs[name]); err != nil {
			return err
		}
	}
	return nil
}

// check checks the evaluations of the teams that go on, those whose grades ran or that have a file,
// then records the grades of the teams of due whose file has no problem, and stops the others.
func (e *evalRun) check(due []string, inputs map[string]evaluation.Inputs) error {
	var names []string
	for _, team := range e.going() {
		if _, err := os.Stat(evaluation.Path(e.labDir, team.Name)); err == nil || slices.Contains(due, team.Name) {
			names = append(names, team.Name)
		}
	}
	slices.Sort(names)
	var checked error
	if len(names) == 0 {
		fmt.Fprintf(e.out, "No evaluation of %s to check in %s\n", e.lab.ID, e.labDir)
	} else {
		problems, err := checkTeams(e.errOut, e.rubric, e.lab, e.labDir, names)
		if err != nil {
			return err
		}
		none := len(problems) == 0
		for _, name := range due {
			if n := problems[name]; n > 0 {
				e.stop(name, stepGrade, plural(n, "problem")+" in "+evaluation.Path(e.labDir, name))
				delete(problems, name) // Counted once, as a team that stopped.
				continue
			}
			if err := e.record(name, stepGrade, inputs[name]); err != nil {
				return err
			}
		}
		if none {
			fmt.Fprintf(e.out, "%s of %s checked in %s: no problem\n", plural(len(names), "file"), e.lab.ID, e.labDir)
		}
		checked = problemsError(problems, len(names))
	}

	var errs []string
	if len(e.stopped) > 0 {
		var stops []string
		for _, team := range e.teams {
			if step, ok := e.stopped[team.Name]; ok {
				stops = append(stops, team.Name+" at the "+stepNames[step])
			}
		}
		errs = append(errs, plural(len(stops), "team")+" stopped: "+strings.Join(stops, ", "))
	}
	if checked != nil {
		errs = append(errs, checked.Error())
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// record records that the step succeeded for the team, with its inputs.
func (e *evalRun) record(team, step string, inputs evaluation.Inputs) error {
	record := e.records[team]
	done := &evaluation.StepRecord{Inputs: inputs, Done: time.Now().Truncate(time.Second)}
	if step == stepReport {
		record.Report = done
	} else {
		record.Grade = done
	}
	if err := evaluation.WriteSteps(e.labDir, team, *record); err != nil {
		return err
	}
	commit := e.facts[team].Commit
	fmt.Fprintf(e.out, "[team: %s] %s: done at %s\n", team, step, commit[:min(len(commit), 7)])
	return nil
}

// stop stops the team at the step, and says why.
func (e *evalRun) stop(team, step, why string) {
	e.stopped[team] = step
	fmt.Fprintf(e.errOut, "[team: %s] stopped at the %s: %s\n", team, stepNames[step], why)
}

// stopAll stops the teams at the step of a failed agent.
func (e *evalRun) stopAll(teams []string, step string, err error) {
	for _, team := range teams {
		e.stop(team, step, err.Error())
	}
}

// going returns the teams that didn't stop, in the order of the registry.
func (e *evalRun) going() []common.Team {
	var teams []common.Team
	for _, team := range e.teams {
		if _, ok := e.stopped[team.Name]; !ok {
			teams = append(teams, team)
		}
	}
	return teams
}

// sameCommit reports whether commit, of at least 7 characters, is the start of the full commit.
func sameCommit(commit, full string) bool {
	return len(commit) >= 7 && strings.HasPrefix(full, commit)
}
