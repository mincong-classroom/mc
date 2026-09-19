package team

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
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
		Long: `Provision one team on GitHub. The command asks for the team, then goes through the steps
one at a time:

  1. create the private repository k8s-{team} from the template repository,
  2. create the secret GitHub team {team},
  3. grant the GitHub team push access to the repository,
  4. add each member to the GitHub team, which invites them to the organization.

Each step is described, with the gh command it runs, and runs only once confirmed. The steps
already done are skipped, so a team can be provisioned again. A team with validation errors (see
"mc team <team> validate") is not provisioned; with warnings, such as a member not among the
students of the registry, the teacher confirms before the steps.

A team that is not in the registry yet is registered on the way, and a registered team without
members gets them: the members are picked among the students of the registry who are not in a
team yet, or typed when there is none to pick, each with their GitHub username. The registry is
updated, except with --dry-run, which describes and confirms the steps but runs nothing.`,
		Example: "  mc team provision\n  mc team provision --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			p := &provisioner{
				client: github.CLI{},
				in:     bufio.NewReader(os.Stdin),
				out:    out,
				dryRun: dryRun,
				color:  isTerminal(out) && os.Getenv("NO_COLOR") == "",
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
	errQuit    = errors.New("quit") // The input is over; the teacher stops the command with ctrl+c
)

// provisioner provisions one team interactively: the teacher confirms each step.
type provisioner struct {
	client github.Client
	in     *bufio.Reader
	out    io.Writer
	dryRun bool
	color  bool // Color the commands, when the output is a terminal
}

// step is one change to make on GitHub to provision a team.
type step struct {
	description string
	done        string // Why the step is already done, "" if it is not
	command     github.Command
}

// isTerminal reports whether the output is a terminal.
func isTerminal(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// run asks for a team and provisions it.
func (p *provisioner) run(load func() (*common.TeamRegistry, error)) error {
	if p.dryRun {
		fmt.Fprintln(p.out, "Dry run: the steps are confirmed, but nothing changes on GitHub.")
	}
	registry, err := load()
	if err != nil {
		return err
	}
	team, ok := p.askTeam(registry)
	if !ok {
		return nil
	}

	fmt.Fprintf(p.out, "\n== Team %s\n", team.Name)
	v := validateTeam(team, registry, p.client)
	printProblems(p.out, "", v)
	if len(v.Errors) > 0 {
		return fmt.Errorf("the team %s is not provisioned: fix the registry, then provision it again", team.Name)
	}
	if len(v.Warnings) > 0 {
		yes, ok := p.confirm("Provision it anyway? (y/N): ")
		if !ok {
			return nil
		}
		if !yes {
			fmt.Fprintf(p.out, "\nTeam %q not provisioned.\n", team.Name)
			return nil
		}
	}

	err = p.provision(team, v.Users)
	switch {
	case errors.Is(err, errSkipped) || errors.Is(err, errQuit):
		fmt.Fprintf(p.out, "\nTeam %q not provisioned.\n", team.Name)
		return nil
	case err != nil:
		return fmt.Errorf("the team %s is not provisioned: %v", team.Name, err)
	case p.dryRun:
		fmt.Fprintf(p.out, "\nDry run: team %q not provisioned, nothing changed on GitHub.\n", team.Name)
	default:
		fmt.Fprintf(p.out, "\nTeam %q provisioned.\n", team.Name)
		fmt.Fprintf(p.out, "- repo: %s\n", github.RepoURL(team.GetRepoName()))
		fmt.Fprintf(p.out, "- team: %s\n", github.TeamURL(team.Name))
	}
	return nil
}

// askTeam asks the teacher which team to provision. A new team is registered, and a registered
// team without members gets them. The teacher stops the command with ctrl+c; it returns false only
// at the end of the input, e.g. of a pipe.
func (p *provisioner) askTeam(registry *common.TeamRegistry) (common.Team, bool) {
	names := make([]string, len(registry.Teams))
	for i, team := range registry.Teams {
		names[i] = team.Name
	}
	if len(names) == 0 {
		fmt.Fprintln(p.out, "\nNo team registered yet.")
	} else {
		fmt.Fprintf(p.out, "\nTeams: %s\n", strings.Join(names, ", "))
	}
	for {
		fmt.Fprint(p.out, "Team to provision: ")
		name, ok := p.readLine()
		if !ok {
			return common.Team{}, false
		}
		if name == "" {
			continue
		}
		var team common.Team
		var err error
		if i := slices.Index(names, name); i >= 0 {
			team, err = p.completeTeam(i, registry)
		} else {
			team, err = p.registerTeam(name, registry)
		}
		if err == nil {
			return team, true
		}
		if errors.Is(err, errQuit) {
			return common.Team{}, false
		}
	}
}

// registerTeam registers a new team, once the teacher confirms, with its members, and saves it to
// the registry. It returns errSkipped when the team is not registered, and errQuit when the input
// is over.
func (p *provisioner) registerTeam(name string, registry *common.TeamRegistry) (common.Team, error) {
	if errs := teamNameErrors(name, registry.Teams); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(p.out, "✗ %s\n", e)
		}
		return common.Team{}, errSkipped
	}
	yes, ok := p.confirm(fmt.Sprintf("%s is not in the registry. Register it? (y/N): ", name))
	if !ok {
		return common.Team{}, errQuit
	}
	if !yes {
		return common.Team{}, errSkipped
	}

	members, err := p.askMembers(registry)
	if err != nil {
		return common.Team{}, err
	}
	team := common.Team{Name: name, Members: members}
	if err := p.save(fmt.Sprintf("the team %s", name), func() error { return common.AddTeam(team) }); err != nil {
		return common.Team{}, errSkipped
	}
	registry.Teams = append(registry.Teams, team)
	return team, nil
}

// completeTeam returns the i-th team of the registry. When it has no members yet, e.g. a team
// created before the course, the teacher can add them, and they are saved to the registry.
func (p *provisioner) completeTeam(i int, registry *common.TeamRegistry) (common.Team, error) {
	team := registry.Teams[i]
	if len(team.Members) > 0 {
		return team, nil
	}
	yes, ok := p.confirm(fmt.Sprintf("%s has no members yet. Add them? (y/N): ", team.Name))
	if !ok {
		return common.Team{}, errQuit
	}
	if !yes {
		return team, nil
	}

	members, err := p.askMembers(registry)
	if err != nil {
		return common.Team{}, err
	}
	if len(members) == 0 {
		return team, nil
	}
	team.Members = members
	if err := p.save(fmt.Sprintf("the members of %s", team.Name), func() error { return common.SetTeamMembers(team.Name, members) }); err != nil {
		return common.Team{}, errSkipped
	}
	registry.Teams[i] = team
	return team, nil
}

// save runs the change of the registry, except in a dry run, and tells the teacher.
func (p *provisioner) save(what string, change func() error) error {
	if p.dryRun {
		fmt.Fprintf(p.out, "(dry run) %s not saved to the registry\n", what)
		return nil
	}
	if err := change(); err != nil {
		fmt.Fprintf(p.out, "✗ cannot save %s: %v\n", what, err)
		return err
	}
	fmt.Fprintf(p.out, "✓ %s saved to %s\n", what, common.TeamRegistryPath())
	return nil
}

// askMembers asks the members of a team, each with their GitHub username: picked by number among
// the students of the registry who are not in a team yet, or typed when there is none to pick.
func (p *provisioner) askMembers(registry *common.TeamRegistry) ([]common.TeamMember, error) {
	var names []string
	var err error
	if students := unassignedStudents(registry.Students, registry.Teams); len(students) > 0 {
		names, err = p.pickStudents(students)
	} else {
		fmt.Fprintln(p.out, "No student to pick in the registry: type the members instead.")
		names, err = p.typeMembers()
	}
	if err != nil {
		return nil, err
	}

	var members []common.TeamMember
	for _, name := range names {
		login, err := p.askGithubUsername(name)
		if err != nil {
			return nil, err
		}
		members = append(members, common.TeamMember{Name: name, Github: login})
	}
	return members, nil
}

// pickStudents asks the teacher to pick the members among the students, by number.
func (p *provisioner) pickStudents(students []common.Student) ([]string, error) {
	fmt.Fprintln(p.out, "Students not in a team yet:")
	for i, student := range students {
		fmt.Fprintf(p.out, "  %2d. %s\n", i+1, student.Name)
	}
	for {
		fmt.Fprint(p.out, "Members, by number, e.g. \"1 2\" (empty for none yet): ")
		line, ok := p.readLine()
		if !ok {
			return nil, errQuit
		}
		picks, err := parsePicks(line, len(students))
		if err != nil {
			fmt.Fprintf(p.out, "✗ %v\n", err)
			continue
		}
		var names []string
		for _, i := range picks {
			names = append(names, students[i-1].Name)
		}
		return names, nil
	}
}

// typeMembers asks the teacher the names of the members, until an empty one.
func (p *provisioner) typeMembers() ([]string, error) {
	var names []string
	for {
		fmt.Fprintf(p.out, "Member %d, \"LAST, First\" (empty when done): ", len(names)+1)
		name, ok := p.readLine()
		if !ok {
			return nil, errQuit
		}
		if name == "" {
			return names, nil
		}
		names = append(names, name)
	}
}

// parsePicks parses numbers between 1 and n, separated by spaces or commas.
func parsePicks(line string, n int) ([]int, error) {
	var picks []int
	for _, field := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == ',' }) {
		i, err := strconv.Atoi(field)
		if err != nil || i < 1 || i > n {
			return nil, fmt.Errorf("%q is not a number between 1 and %d", field, n)
		}
		if slices.Contains(picks, i) {
			return nil, fmt.Errorf("%d is picked twice", i)
		}
		picks = append(picks, i)
	}
	return picks, nil
}

// askGithubUsername asks the GitHub username of the student until it exists, and prints its
// display name, for the student to confirm it.
func (p *provisioner) askGithubUsername(name string) (string, error) {
	for {
		fmt.Fprintf(p.out, "GitHub username of %s: ", name)
		line, ok := p.readLine()
		if !ok {
			return "", errQuit
		}
		login := strings.TrimPrefix(line, "@")
		if login == "" {
			continue
		}
		user, err := p.client.GetUser(login)
		switch {
		case err != nil:
			fmt.Fprintf(p.out, "✗ cannot check the GitHub user @%s: %v\n", login, err)
		case user == nil:
			fmt.Fprintf(p.out, "✗ the GitHub user @%s does not exist\n", login)
		default:
			fmt.Fprintf(p.out, "  @%s: %s\n", login, describeGithubUser(user))
			return login, nil
		}
	}
}

// provision runs the steps of one team, each once confirmed. It returns errSkipped when the
// teacher declines a step, errQuit when the input is over, or the error of the step that failed.
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
		fmt.Fprintf(p.out, "      %s\n", p.commandColor("$ "+s.command.String()))

		yes, ok := p.confirm("      Run it? (y/N): ")
		if !ok {
			return errQuit
		}
		if !yes {
			fmt.Fprintln(p.out, "      Skipped, with the remaining steps of the team")
			return errSkipped
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
	return nil
}

// commandColor renders a command that mc runs in dark yellow, when the output is colored.
func (p *provisioner) commandColor(text string) string {
	if !p.color {
		return text
	}
	return "\033[33m" + text + "\033[0m"
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

// confirm asks a yes or no question, "no" by default. It returns false once the input is over.
func (p *provisioner) confirm(question string) (yes bool, ok bool) {
	for {
		fmt.Fprint(p.out, question)
		line, ok := p.readLine()
		if !ok {
			return false, false
		}
		switch strings.ToLower(line) {
		case "y", "yes":
			return true, true
		case "", "n", "no":
			return false, true
		}
	}
}

// readLine reads a line of the teacher's input, trimmed. It returns false once the input is over,
// e.g. at the end of a pipe.
func (p *provisioner) readLine() (string, bool) {
	line, err := p.in.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		fmt.Fprintln(p.out)
		return "", false
	}
	return line, true
}
