package team

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

// newProvisionCmd returns the command provisioning the team with the given name, or all the teams
// when the name is empty.
func newProvisionCmd(name string) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Create the repository and the GitHub team of the team, and invite its members",
		Long: `Provision the team on GitHub, one step at a time:

  1. create the private repository k8s-{team} from the template repository,
  2. create the secret GitHub team {team},
  3. grant the GitHub team push access to the repository,
  4. add each member to the GitHub team, which invites them to the organization.

Each step is described, with the gh command it runs, and runs only once confirmed. The steps
already done are skipped, so the command can run again, e.g. once the members of a team created
before the course are known. A team that is not valid (see "mc team <team> validate") is not
provisioned. With --dry-run, the steps are described and confirmed, but nothing runs.`,
		Example: "  mc team " + name + " provision\n  mc team " + name + " provision --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProvision(cmd.OutOrStdout(), name, dryRun)
		},
		SilenceUsage: true,
	}
	if name == "" {
		cmd.Short = "Create the repository and the GitHub team of every team, and invite the members"
		cmd.Long = strings.Replace(cmd.Long, "Provision the team on GitHub", "Provision every team of the registry on GitHub", 1)
		cmd.Example = "  mc team provision\n  mc team provision --dry-run"
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Describe and confirm the steps, but do not run them")
	return cmd
}

var (
	errSkipped = errors.New("skipped")
	errQuit    = errors.New("quit")
)

type answer int

const (
	answerYes answer = iota
	answerNo
	answerQuit
)

// provisioner provisions the teams interactively: it asks the teacher to confirm each step.
type provisioner struct {
	client   github.Client
	in       *bufio.Reader
	out      io.Writer
	dryRun   bool
	yesToAll bool // Set when the teacher answers "all"
}

// step is one change to make on GitHub to provision a team.
type step struct {
	description string
	done        string // Why the step is already done, "" if it is not
	command     github.Command
}

func runProvision(out io.Writer, name string, dryRun bool) error {
	teams, selected, err := loadTeams(name)
	if err != nil {
		return err
	}
	students, err := loadStudents(out)
	if err != nil {
		return err
	}

	p := &provisioner{
		client: github.CLI{},
		in:     bufio.NewReader(os.Stdin),
		out:    out,
		dryRun: dryRun,
	}
	if dryRun {
		fmt.Fprintln(out, "Dry run: the steps are confirmed, but nothing changes on GitHub.")
	}

	var provisioned, skipped, failed int
	for i, team := range selected {
		fmt.Fprintf(out, "\n== Team %s (%d/%d)\n", team.Name, i+1, len(selected))
		v := validateTeam(team, teams, students, p.client)
		if len(v.Problems) > 0 {
			for _, problem := range v.Problems {
				fmt.Fprintf(out, "✗ %s\n", problem)
			}
			fmt.Fprintln(out, "Not provisioned: fix the registry, then run the command again.")
			failed++
			continue
		}

		err := p.provision(team, v.Users)
		switch {
		case err == nil:
			provisioned++
		case errors.Is(err, errSkipped):
			skipped++
		case errors.Is(err, errQuit):
			skipped += len(selected) - i
		default:
			failed++
		}
		if errors.Is(err, errQuit) {
			break
		}
	}

	if len(selected) > 1 {
		fmt.Fprintf(out, "\nProvisioned: %d · skipped: %d · failed: %d\n", provisioned, skipped, failed)
	}
	if failed > 0 {
		return fmt.Errorf("%d team(s) not provisioned", failed)
	}
	return nil
}

// provision runs the steps of one team. It returns errSkipped or errQuit when the teacher
// declines a step, or the error of the step that failed.
func (p *provisioner) provision(team common.Team, users map[string]*github.User) error {
	steps, err := p.steps(team, users)
	if err != nil {
		fmt.Fprintf(p.out, "✗ cannot get the status of the team: %v\n", err)
		return err
	}

	for i, s := range steps {
		fmt.Fprintf(p.out, "[%d/%d] %s\n", i+1, len(steps), s.description)
		if s.done != "" {
			fmt.Fprintf(p.out, "      ✓ %s\n", s.done)
			continue
		}
		fmt.Fprintf(p.out, "      $ %s\n", s.command)

		switch p.ask() {
		case answerNo:
			fmt.Fprintln(p.out, "      Skipped, with the remaining steps of the team")
			return errSkipped
		case answerQuit:
			return errQuit
		}
		if p.dryRun {
			fmt.Fprintln(p.out, "      (dry run) not run")
			continue
		}
		if err := p.client.Run(s.command); err != nil {
			fmt.Fprintf(p.out, "      ✗ %v\n", err)
			return err
		}
		fmt.Fprintln(p.out, "      ✓ done")
	}

	if !p.dryRun {
		fmt.Fprintf(p.out, "Status: %s\n", teamStatus(team, p.client).Summary())
	}
	return nil
}

// steps returns the steps to provision the team, and whether each one is already done.
func (p *provisioner) steps(team common.Team, users map[string]*github.User) ([]step, error) {
	status := teamStatus(team, p.client)
	if status.Err != nil {
		return nil, status.Err
	}
	repo := team.GetRepoName()

	steps := []step{
		{
			description: fmt.Sprintf("Create the private repository %s/%s from the template %s", github.Org, repo, github.TemplateRepo),
			done:        doneIf(status.RepoExists, "the repository exists"),
			command:     github.CreateRepoCommand(repo),
		},
		{
			description: fmt.Sprintf("Create the secret GitHub team %s", team.Name),
			done:        doneIf(status.TeamExists, "the GitHub team exists"),
			command:     github.CreateTeamCommand(team.Name),
		},
		{
			description: fmt.Sprintf("Grant the GitHub team %s push access to %s", team.Name, repo),
			done:        doneIf(status.RepoRole == "write", "the GitHub team has push access"),
			command:     github.GrantPushCommand(team.Name, repo),
		},
	}
	for _, member := range team.Members {
		state := status.States[member.Github]
		steps = append(steps, step{
			description: fmt.Sprintf("Add %s, %s, to the GitHub team %s: GitHub invites them to the organization by email",
				describeMember(member), describeGithubUser(users[member.Github]), team.Name),
			done: doneIf(state == "active", "already a member") +
				doneIf(state == "pending", "already invited (pending)"),
			command: github.AddMemberCommand(team.Name, member.Github),
		})
	}
	return steps, nil
}

func doneIf(done bool, reason string) string {
	if done {
		return reason
	}
	return ""
}

// ask asks the teacher whether to run the current step.
func (p *provisioner) ask() answer {
	if p.yesToAll {
		return answerYes
	}
	for {
		fmt.Fprint(p.out, "      Run it? [y]es, [n]o (skip the team), [a]ll (yes to everything), [q]uit: ")
		line, err := p.in.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return answerYes
		case "a", "all":
			p.yesToAll = true
			return answerYes
		case "n", "no":
			return answerNo
		case "q", "quit":
			return answerQuit
		}
		if err != nil { // No more input, e.g. end of a pipe
			fmt.Fprintln(p.out)
			return answerQuit
		}
	}
}
