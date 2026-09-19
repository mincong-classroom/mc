package team

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

func newProvisionCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a team on GitHub: its repository, its GitHub team and its members",
		Long: `Provision a team on GitHub. The command asks for the team, then goes through the steps
one at a time:

  1. create the private repository k8s-{team} from the template repository,
  2. create the secret GitHub team {team},
  3. grant the GitHub team push access to the repository,
  4. add each member to the GitHub team, which invites them to the organization.

Each step is described, with the gh command it runs, and runs only once confirmed. The steps
already done are skipped, so a team can be provisioned again, e.g. once the members of a team
created before the course are known. A team that is not valid (see "mc team <team> validate")
is not provisioned. Once a team is done, the command asks for the next one, and reads the
registry again, so it can be edited in between. With --dry-run, the steps are described and
confirmed, but nothing runs.`,
		Example: "  mc team provision\n  mc team provision --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := &provisioner{
				client: github.CLI{},
				in:     bufio.NewReader(os.Stdin),
				out:    cmd.OutOrStdout(),
				dryRun: dryRun,
			}
			return p.run(loadRegistry)
		},
		SilenceUsage: true,
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
	yesToAll bool // Set when the teacher answers "all", until the end of the team
}

// step is one change to make on GitHub to provision a team.
type step struct {
	description string
	done        string // Why the step is already done, "" if it is not
	command     github.Command
}

// loadRegistry reads the team registry and the school list, which is nil when it does not exist.
func loadRegistry(out io.Writer) ([]common.Team, []common.Student, error) {
	teams, err := common.ListTeams()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list teams: %v", err)
	}
	students, err := loadStudents(out)
	return teams, students, err
}

// run asks for a team and provisions it, until the teacher quits. The registry is loaded again
// before each team.
func (p *provisioner) run(load func(out io.Writer) ([]common.Team, []common.Student, error)) error {
	if p.dryRun {
		fmt.Fprintln(p.out, "Dry run: the steps are confirmed, but nothing changes on GitHub.")
	}
	failed := 0
	for round := 0; ; round++ {
		notes := p.out
		if round > 0 {
			notes = io.Discard // The notes about the registry were printed the first time
		}
		teams, students, err := load(notes)
		if err != nil {
			return err
		}
		team, ok := p.askTeam(teams)
		if !ok {
			break
		}

		fmt.Fprintf(p.out, "\n== Team %s\n", team.Name)
		v := validateTeam(team, teams, students, p.client)
		if len(v.Problems) > 0 {
			for _, problem := range v.Problems {
				fmt.Fprintf(p.out, "✗ %s\n", problem)
			}
			fmt.Fprintln(p.out, "Not provisioned: fix the registry, then provision the team again.")
			failed++
			continue
		}
		err = p.provision(team, v.Users)
		if errors.Is(err, errQuit) {
			break
		}
		if err != nil && !errors.Is(err, errSkipped) {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d team(s) not provisioned", failed)
	}
	return nil
}

// askTeam asks the teacher which team to provision. It returns false when the teacher quits.
func (p *provisioner) askTeam(teams []common.Team) (common.Team, bool) {
	names := make([]string, len(teams))
	for i, team := range teams {
		names[i] = team.Name
	}
	fmt.Fprintf(p.out, "\nTeams: %s\n", strings.Join(names, ", "))
	for {
		fmt.Fprint(p.out, "Team to provision (empty to quit): ")
		line, err := p.in.ReadString('\n')
		name := strings.TrimSpace(line)
		if i := slices.Index(names, name); name != "" && i >= 0 {
			return teams[i], true
		}
		if name == "" || err != nil {
			if err != nil { // No more input, e.g. end of a pipe
				fmt.Fprintln(p.out)
			}
			return common.Team{}, false
		}
		fmt.Fprintf(p.out, "Unknown team %q.\n", name)
	}
}

// provision runs the steps of one team. It returns errSkipped or errQuit when the teacher
// declines a step, or the error of the step that failed.
func (p *provisioner) provision(team common.Team, users map[string]*github.User) error {
	p.yesToAll = false // "all" applies to the steps of one team only
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
		fmt.Fprint(p.out, "      Run it? [y]es, [n]o (skip the team), [a]ll (yes to the next steps of the team), [q]uit: ")
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
