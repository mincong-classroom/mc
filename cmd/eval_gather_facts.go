package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/github"
	"github.com/mincong-classroom/mc/rubric"
)

var (
	gatherFactsLab      string
	gatherFactsTeams    []string
	gatherFactsLate     bool
	gatherFactsRev      string
	gatherFactsTemplate string
	gatherFactsDir      string
	gatherFactsForce    bool
)

var evalGatherFactsCmd = &cobra.Command{
	Use:   "gather-facts",
	Short: "Gather the facts of each rule of a lab, in a file on the local branch teacher of each team",
	Long: `Gather the facts of each rule of a lab for the teams, in a file for the teacher: what the team
wrote in its report for the exercise, without the statement; the files of the rule and those the
report links to, imported when they are short, or as their changes since the template; and the
evidence that the evaluator made, in the directory next to the report, such as docs/lab-1.evidence/:
its notes {RULE_ID}.md, and its files {rule-id}-*, such as l1-dkf-build.log. It also reads the
images of the rules on Docker Hub. It gathers facts, not a score. The rules are those of the
year, from the key rules of ~/.mc/settings.yaml, or the file of --rules-file.

The commit evaluated is the head of main at the deadline of the lab, from the pushes of the
activity of the repository on GitHub, or origin/main with --late.

The facts, such as docs/lab-1.facts.md, go to the local branch teacher of the clone of the team,
checked out in a worktree of --dir: the branch starts from the commit evaluated, the next pass
merges the new one into it, and a change of the facts is committed. The branch is never pushed.
Run it again once the evaluator adds its files.

The front matter of the facts holds the commit evaluated and the commit of mc, then the images of
each rule on Docker Hub and the number of screenshots of the team, which mc eval sync writes in the
evaluation sheet. The evaluator copies it into the report that it writes from the facts, such as
docs/lab-1.report.md. While both commits and the images stay the same, the report is up to date:
the team is skipped, unless --force is given, or mc is a local build with changes.

The files of the former names, docs/lab-1.evidence.md for the facts and docs/lab-1.generated.md
for the report, are renamed with git mv at the next pass, and the renames are committed.`,
	Args: cobra.NoArgs,
	RunE: runGatherFacts,
	Example: `  mc eval gather-facts -l L1 --template ~/github/mincong-classroom/containers
  mc eval gather-facts -l L1 -t red --late
  mc eval gather-facts -l L1 -t red --force`,
	SilenceUsage: true,
}

func init() {
	// --dir shadows the one of mc eval, the directory of the evaluations: here, it is the one of the
	// worktrees.
	evalGatherFactsCmd.Flags().StringVarP(&gatherFactsLab, "lab", "l", "", "The lab, e.g. L1 or 1")
	evalGatherFactsCmd.Flags().StringArrayVarP(&gatherFactsTeams, "team", "t", nil, "A team, repeatable (default: every team of the registry)")
	evalGatherFactsCmd.Flags().BoolVar(&gatherFactsLate, "late", false, "Evaluate origin/main, for a late pass, instead of the head of main at the deadline")
	evalGatherFactsCmd.Flags().StringVar(&gatherFactsRev, "rev", "", "Evaluate this commit instead, for one team")
	evalGatherFactsCmd.Flags().StringVar(&gatherFactsTemplate, "template", "", "The clone of the template of the team repositories, to tell what a team wrote (default: the first commit of each repository)")
	evalGatherFactsCmd.Flags().StringVar(&gatherFactsDir, "dir", "", "The directory of the worktrees, one per team (default: ~/.mc/{year}/teacher)")
	evalGatherFactsCmd.Flags().BoolVar(&gatherFactsForce, "force", false, "Gather the facts again, even when the report is up to date")
	_ = evalGatherFactsCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	evalCmd.AddCommand(evalGatherFactsCmd)
}

func runGatherFacts(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(evalRules)
	if err != nil {
		return err
	}
	r, err := loadAndCheck(path, "", cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	labID := rubric.LabID(gatherFactsLab)
	if _, ok := r.Lab(labID); !ok {
		return fmt.Errorf("no lab %q in %s", gatherFactsLab, path)
	}
	if gatherFactsLate && gatherFactsRev != "" {
		return errors.New("--late and --rev are exclusive")
	}
	teams, err := common.ListTeams()
	if err != nil {
		return err
	}
	if len(gatherFactsTeams) > 0 {
		if teams, err = common.FilterTeams(teams, gatherFactsTeams); err != nil {
			return err
		}
	}
	if gatherFactsRev != "" && len(teams) != 1 {
		return errors.New("--rev evaluates the commit of one team: give it with -t")
	}
	dir := gatherFactsDir
	if dir == "" {
		dir = teacherDir()
	}
	pass := factsPass{
		rubric:   r,
		lab:      labID,
		dir:      dir,
		late:     gatherFactsLate,
		rev:      gatherFactsRev,
		template: gatherFactsTemplate,
		force:    gatherFactsForce,
	}
	failed, err := pass.gather(cmd.OutOrStdout(), teams)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if err, ok := failed[team.Name]; ok {
			fmt.Fprintf(cmd.ErrOrStderr(), "[team: %s] %v\n", team.Name, err)
		}
	}
	if len(failed) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d of %d teams failed\n", len(failed), len(teams))
		os.Exit(1)
	}
	return nil
}

// factsPass is a pass of the facts of a lab, for mc eval gather-facts and the first step of mc
// eval run.
type factsPass struct {
	rubric   *rubric.Rubric
	lab      string // The ID of the lab, such as L1.
	dir      string // The directory of the worktrees of the branch teacher, one per team.
	late     bool   // Gather the facts of origin/main, for a late pass.
	rev      string // Gather the facts of this commit instead, for one team.
	template string // The clone of the template of the team repositories, if any.
	force    bool   // Gather the facts again, even when the report is up to date.
	prefix   string // The start of each line after the team, such as "facts: " in mc eval run.
	// forceFlag is the flag that gathers the facts again: --force by default.
	forceFlag string
}

// gather gathers the facts of each team, prints one line per team, and returns the error of each
// team that failed, by its name. Its error is one that stops every team, such as a broken template.
func (p factsPass) gather(out io.Writer, teams []common.Team) (map[string]error, error) {
	revision := common.Revision()
	opts := evidence.Options{
		Rubric:   p.rubric,
		Lab:      p.lab,
		Dir:      p.dir,
		Late:     p.late,
		Rev:      p.rev,
		Pushes:   github.CLI{},
		Registry: evidence.DockerHub{},
		Revision: revision,
		// A local build with changes doesn't tell which code ran: its reports are never up to date.
		Force: p.force || !common.StableRevision(revision),
	}
	if p.template != "" {
		var err error
		if opts.Template, err = evidence.LoadTemplate(p.template); err != nil {
			return nil, err
		}
	}

	forceFlag := p.forceFlag
	if forceFlag == "" {
		forceFlag = "--force"
	}
	failed := map[string]error{}
	for _, team := range teams {
		result, err := evidence.Gather(evidence.Team{Name: team.Name, Repo: team.GetRepoName(), Clone: team.GetRepoPath()}, opts)
		if err != nil {
			failed[team.Name] = err
			continue
		}
		for _, r := range result.Renamed {
			fmt.Fprintf(out, "[team: %s] %srenamed %s to %s\n", team.Name, p.prefix, r.From, r.To)
		}
		if result.UpToDate {
			fmt.Fprintf(out, "[team: %s] %s%s, %s: %s, up to date with mc %s (%s to gather again)\n", team.Name, p.prefix, result.Commit[:7], result.How, result.File, revision, forceFlag)
			continue
		}
		change := "unchanged"
		if result.Committed {
			change = "committed to the branch " + evidence.Branch
		}
		fmt.Fprintf(out, "[team: %s] %s%s, %s: %s, %s\n", team.Name, p.prefix, result.Commit[:7], result.How, result.File, change)
	}
	return failed, nil
}
