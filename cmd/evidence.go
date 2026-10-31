package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/evidence"
	"github.com/mincong-classroom/mc/github"
	"github.com/mincong-classroom/mc/rubric"
)

var (
	evidenceLab      string
	evidenceTeams    []string
	evidenceLate     bool
	evidenceRev      string
	evidenceTemplate string
	evidenceDir      string
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence <rules.yaml>",
	Short: "Gather the evidence of each rule of a lab, in a report on the local branch teacher of each team",
	Long: `Gather the evidence of each rule of a lab for the teams, in a report for the teacher: what the
team wrote in its report for the exercise, without the statement; the files of the rule and those
the report links to, imported when they are short, or as their changes since the template; and
what the evaluator made, in the directory next to the report, such as docs/lab-1.evidence/:
its notes {RULE_ID}.md, and its files {rule-id}-*, such as l1-dkf-build.log. It also reads the
images of the rules on Docker Hub. It gathers facts, not a score.

The commit evaluated is the head of main at the deadline of the lab, from the pushes of the
activity of the repository on GitHub, or origin/main with --late.

The report, such as docs/lab-1.evidence.md, goes to the local branch teacher of the clone of the
team, checked out in a worktree of --dir: the branch starts from the commit evaluated, the next
pass merges the new one into it, and a change of the report is committed. The branch is never
pushed. Run it again once the evaluator adds its files.`,
	Args: cobra.ExactArgs(1),
	RunE: runEvidence,
	Example: `  mc evidence evaluation-rules.yaml -l L1 --template ~/github/mincong-classroom/containers
  mc evidence evaluation-rules.yaml -l L1 -t red --late`,
	SilenceUsage: true,
}

func init() {
	evidenceCmd.Flags().StringVarP(&evidenceLab, "lab", "l", "", "The lab, e.g. L1 or 1")
	evidenceCmd.Flags().StringArrayVarP(&evidenceTeams, "team", "t", nil, "A team, repeatable (default: every team of the registry)")
	evidenceCmd.Flags().BoolVar(&evidenceLate, "late", false, "Evaluate origin/main, for a late pass, instead of the head of main at the deadline")
	evidenceCmd.Flags().StringVar(&evidenceRev, "rev", "", "Evaluate this commit instead, for one team")
	evidenceCmd.Flags().StringVar(&evidenceTemplate, "template", "", "The clone of the template of the team repositories, to tell what a team wrote (default: the first commit of each repository)")
	evidenceCmd.Flags().StringVar(&evidenceDir, "dir", "", "The directory of the worktrees, one per team (default: ~/.mc/{year}/teacher)")
	_ = evidenceCmd.MarkFlagRequired("lab") // The flag exists: it is defined above.
	RootCmd.AddCommand(evidenceCmd)
}

func runEvidence(cmd *cobra.Command, args []string) error {
	r, err := loadAndCheck(args[0], "", cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	labID := rubric.LabID(evidenceLab)
	if _, ok := r.Lab(labID); !ok {
		return fmt.Errorf("no lab %q in %s", evidenceLab, args[0])
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
		dir = filepath.Join(common.ConfigDir(), strconv.Itoa(common.Year()), "teacher")
	}
	opts := evidence.Options{
		Rubric:   r,
		Lab:      labID,
		Dir:      dir,
		Late:     evidenceLate,
		Rev:      evidenceRev,
		Pushes:   github.CLI{},
		Registry: evidence.DockerHub{},
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
