package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/github"
	"github.com/mincong-classroom/mc/rubric"
)

var (
	evidenceRules    string
	evidenceLab      string
	evidenceTeams    []string
	evidenceLate     bool
	evidenceRev      string
	evidenceTemplate string
	evidenceDir      string
	evidenceForce    bool
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Gather the evidence of each rule of a lab, in a report on the local branch teacher of each team",
	Long: `Gather the evidence of each rule of a lab for the teams, in a report for the teacher: what the
team wrote in its report for the exercise, without the statement; the files of the rule and those
the report links to, imported when they are short, or as their changes since the template; and
what the evaluator made, in the directory next to the report, such as docs/lab-1.evidence/:
its notes {RULE_ID}.md, and its files {rule-id}-*, such as l1-dkf-build.log. It also reads the
images of the rules on Docker Hub. It gathers facts, not a score. The rules are those of the
year, from the key rules of ~/.mc/settings.yaml, or the file of --rules-file.

The commit evaluated is the head of main at the deadline of the lab, from the pushes of the
activity of the repository on GitHub, or origin/main with --late.

The report, such as docs/lab-1.evidence.md, goes to the local branch teacher of the clone of the
team, checked out in a worktree of --dir: the branch starts from the commit evaluated, the next
pass merges the new one into it, and a change of the report is committed. The branch is never
pushed. Run it again once the evaluator adds its files.

The front matter of the report holds the commit evaluated and the commit of mc. The evaluator
copies it into the report that it generates from the evidence, such as docs/lab-1.generated.md.
While both commits stay the same, the generated report is up to date: the team is skipped,
unless --force is given, or mc is a local build with changes.`,
	Args: cobra.NoArgs,
	RunE: runEvidence,
	Example: `  mc evidence -l L1 --template ~/github/mincong-classroom/containers
  mc evidence -l L1 -t red --late
  mc evidence -l L1 -t red --force`,
	SilenceUsage: true,
}

func init() {
	evidenceCmd.Flags().StringVar(&evidenceRules, "rules-file", "", rulesFileUsage)
	evidenceCmd.Flags().StringVarP(&evidenceLab, "lab", "l", "", "The lab, e.g. L1 or 1")
	evidenceCmd.Flags().StringArrayVarP(&evidenceTeams, "team", "t", nil, "A team, repeatable (default: every team of the registry)")
	evidenceCmd.Flags().BoolVar(&evidenceLate, "late", false, "Evaluate origin/main, for a late pass, instead of the head of main at the deadline")
	evidenceCmd.Flags().StringVar(&evidenceRev, "rev", "", "Evaluate this commit instead, for one team")
	evidenceCmd.Flags().StringVar(&evidenceTemplate, "template", "", "The clone of the template of the team repositories, to tell what a team wrote (default: the first commit of each repository)")
	evidenceCmd.Flags().StringVar(&evidenceDir, "dir", "", "The directory of the worktrees, one per team (default: ~/.mc/{year}/teacher)")
	evidenceCmd.Flags().BoolVar(&evidenceForce, "force", false, "Gather the evidence again, even when the generated report is up to date")
	_ = evidenceCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	RootCmd.AddCommand(evidenceCmd)
}

func runEvidence(cmd *cobra.Command, _ []string) error {
	path, err := common.RulesPath(evidenceRules)
	if err != nil {
		return err
	}
	r, err := loadAndCheck(path, "", cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	labID := rubric.LabID(evidenceLab)
	if _, ok := r.Lab(labID); !ok {
		return fmt.Errorf("no lab %q in %s", evidenceLab, path)
	}
	if evidenceLate && evidenceRev != "" {
		return errors.New("--late and --rev are exclusive")
	}
	teams, err := common.ListTeams()
	if err != nil {
		return err
	}
	if len(evidenceTeams) > 0 {
		if teams, err = common.FilterTeams(teams, evidenceTeams); err != nil {
			return err
		}
	}
	if evidenceRev != "" && len(teams) != 1 {
		return errors.New("--rev evaluates the commit of one team: give it with -t")
	}
	dir := evidenceDir
	if dir == "" {
		dir = teacherDir()
	}
	revision := common.Revision()
	opts := evidence.Options{
		Rubric:   r,
		Lab:      labID,
		Dir:      dir,
		Late:     evidenceLate,
		Rev:      evidenceRev,
		Pushes:   github.CLI{},
		Registry: evidence.DockerHub{},
		Revision: revision,
		// A local build with changes doesn't tell which code ran: its reports are never up to date.
		Force: evidenceForce || !common.StableRevision(revision),
	}
	if evidenceTemplate != "" {
		if opts.Template, err = evidence.LoadTemplate(evidenceTemplate); err != nil {
			return err
		}
	}

	out := cmd.OutOrStdout()
	failed := 0
	for _, team := range teams {
		result, err := evidence.Gather(evidence.Team{Name: team.Name, Repo: team.GetRepoName(), Clone: team.GetRepoPath()}, opts)
		if err != nil {
			failed++
			fmt.Fprintf(cmd.ErrOrStderr(), "[team: %s] %v\n", team.Name, err)
			continue
		}
		if result.UpToDate {
			fmt.Fprintf(out, "[team: %s] %s, %s: %s, up to date with mc %s (--force to gather again)\n", team.Name, result.Commit[:7], result.How, result.Report, revision)
			continue
		}
		change := "unchanged"
		if result.Committed {
			change = "committed to the branch " + evidence.Branch
		}
		fmt.Fprintf(out, "[team: %s] %s, %s: %s, %s\n", team.Name, result.Commit[:7], result.How, result.Report, change)
	}
	if failed > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d of %d teams failed\n", failed, len(teams))
		os.Exit(1)
	}
	return nil
}
