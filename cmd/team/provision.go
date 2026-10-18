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

func newProvisionCmd(name string) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision the team on GitHub: its repository, its GitHub team and its members",
		Long: `Provision the team on GitHub: create its private repository k8s-{team} from the template
repository and its secret GitHub team, grant the GitHub team push access to the repository, and
add each member to the GitHub team, which invites them to the organization.

The command shows the members of the team, each with the display name of their GitHub account,
and asks once to confirm them. Then it runs the steps that are not done yet, without asking again:
a team can be provisioned again. A team with validation errors (see "mc team <team> validate") is
not provisioned; its warnings, such as a member not among the students of the registry, are shown
before the confirmation.

The team does not have to be in the registry yet. A new team, or a registered team without members,
gets them first: the members are picked among the students of the registry who are not in a team
yet, or typed when there is none to pick, each with their GitHub username. The registry is updated
once confirmed, except with --dry-run, which asks the same questions but changes nothing.`,
		Example: "  mc team " + name + " provision\n  mc team " + name + " provision --dry-run",
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
			return p.run(name, loadRegistry)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Ask the same questions, but change nothing on GitHub nor in the registry")
	return cmd
}

var errQuit = errors.New("quit") // The input is over; the teacher stops the command with ctrl+c

// provisioner provisions one team: the teacher confirms its members once, then the steps run.
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
	command     github.Command
}

// registryChange is a change of the team registry, saved once the teacher confirms the team.
type registryChange struct {
	what  string // What is saved, such as "the team purple"
	apply func() error
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

// run provisions the team with the given name, in the registry or not.
func (p *provisioner) run(name string, load func() (*common.TeamRegistry, error)) error {
	if p.dryRun {
		fmt.Fprintln(p.out, "Dry run: nothing changes on GitHub, nor in the registry.")
	}
	registry, err := load()
	if err != nil {
		return err
	}
	team, change, err := p.prepare(name, registry)
	if errors.Is(err, errQuit) {
		fmt.Fprintf(p.out, "\nTeam %q not provisioned.\n", name)
		return nil
	}
	if err != nil {
		return err
	}

	v := validateTeam(team, registry, p.client)
	status := teamStatus(team, p.client)
	fmt.Fprintf(p.out, "\n== Team %s: %s\n", team.Name, describeMemberCount(team))
	printMembers(p.out, v, status.States)
	printProblems(p.out, "", v)
	if len(v.Errors) > 0 {
		return fmt.Errorf("the team %s is not provisioned: fix the registry, then provision it again", team.Name)
	}
	if status.Err != nil {
		return fmt.Errorf("the team %s is not provisioned: cannot get its status on GitHub: %v", team.Name, status.Err)
	}
	steps := stepsToRun(team, status)
	if len(steps) == 0 && change == nil {
		fmt.Fprintf(p.out, "\nTeam %q is already provisioned.\n", team.Name)
		p.printLinks(team)
		return nil
	}

	yes, ok := p.confirm(fmt.Sprintf("Provision the team %s? (y/N): ", team.Name))
	if !ok || !yes {
		fmt.Fprintf(p.out, "\nTeam %q not provisioned.\n", team.Name)
		return nil
	}
	if change != nil {
		if err := p.save(change); err != nil {
			return fmt.Errorf("the team %s is not provisioned: %v", team.Name, err)
		}
	}
	if err := p.provision(steps); err != nil {
		return fmt.Errorf("the team %s is not fully provisioned, provision it again: %v", team.Name, err)
	}
	if p.dryRun {
		fmt.Fprintf(p.out, "\nDry run: team %q not provisioned, nothing changed on GitHub.\n", team.Name)
		return nil
	}
	fmt.Fprintf(p.out, "\nTeam %q provisioned.\n", team.Name)
	p.printLinks(team)
	return nil
}

func (p *provisioner) printLinks(team common.Team) {
	fmt.Fprintf(p.out, "- repo: %s\n", github.RepoURL(team.GetRepoName()))
	fmt.Fprintf(p.out, "- team: %s\n", github.TeamURL(team.Name))
}

// prepare returns the team to provision. A team that is not in the registry, or that has no
// members yet, gets its members, with the change of the registry to save once the teacher confirms
// the team; the registry in memory is changed at once, for the validation. It returns errQuit when
// the input is over.
func (p *provisioner) prepare(name string, registry *common.TeamRegistry) (common.Team, *registryChange, error) {
	i := slices.IndexFunc(registry.Teams, func(team common.Team) bool { return team.Name == name })
	if i >= 0 && len(registry.Teams[i].Members) > 0 {
		return registry.Teams[i], nil, nil
	}

	if errs := teamNameErrors(name, registry.Teams); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(p.out, "✗ %s\n", e)
		}
		return common.Team{}, nil, fmt.Errorf("the team %s is not provisioned: fix its name", name)
	}
	if i < 0 {
		fmt.Fprintf(p.out, "The team %s is not in the registry yet.\n", name)
	} else {
		fmt.Fprintf(p.out, "The team %s has no members yet.\n", name)
	}
	members, err := p.askMembers(registry)
	if err != nil {
		return common.Team{}, nil, err
	}

	if i < 0 {
		team := common.Team{Name: name, Members: members}
		registry.Teams = append(registry.Teams, team)
		return team, &registryChange{
			what:  "the team " + name,
			apply: func() error { return common.AddTeam(team) },
		}, nil
	}
	team := registry.Teams[i]
	if len(members) == 0 {
		return team, nil, nil
	}
	team.Members = members
	registry.Teams[i] = team
	return team, &registryChange{
		what:  "the members of " + name,
		apply: func() error { return common.SetTeamMembers(name, members) },
	}, nil
}

// printMembers prints each member of the team with their GitHub account, for the students to
// confirm it, and whether they are in the GitHub team already.
func printMembers(out io.Writer, v Validation, states map[string]string) {
	for _, member := range v.Team.Members {
		line := fmt.Sprintf("  - %s: %s", describeMember(member), describeGithubUser(v.Users[member.Github]))
		switch states[member.Github] {
		case "active":
			line += ", already a member"
		case "pending":
			line += ", already invited"
		}
		fmt.Fprintln(out, line)
	}
}

// save saves the change of the registry, except in a dry run, and tells the teacher.
func (p *provisioner) save(change *registryChange) error {
	if p.dryRun {
		fmt.Fprintf(p.out, "(dry run) %s not saved to the registry\n", change.what)
		return nil
	}
	if err := change.apply(); err != nil {
		fmt.Fprintf(p.out, "✗ cannot save %s: %v\n", change.what, err)
		return err
	}
	fmt.Fprintf(p.out, "✓ %s saved to %s\n", change.what, common.TeamRegistryPath())
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

// typeMembers asks the teacher the names of the members, in the format "LAST First", until an
// empty one.
func (p *provisioner) typeMembers() ([]string, error) {
	var names []string
	for {
		fmt.Fprintf(p.out, "Member %d, \"LAST First\" (empty when done): ", len(names)+1)
		line, ok := p.readLine()
		if !ok {
			return nil, errQuit
		}
		if line == "" {
			return names, nil
		}
		name, err := common.ParseName(line)
		if err != nil {
			fmt.Fprintf(p.out, "✗ %v\n", err)
			continue
		}
		names = append(names, name.String())
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

// provision runs the steps, or describes them in a dry run. It stops at the first step that fails,
// and returns its error.
func (p *provisioner) provision(steps []step) error {
	for _, s := range steps {
		if p.dryRun {
			fmt.Fprintf(p.out, "(dry run) %s\n", s.description)
			fmt.Fprintf(p.out, "          %s\n", p.commandColor("$ "+s.command.String()))
			continue
		}
		if err := p.client.Run(s.command); err != nil {
			fmt.Fprintf(p.out, "✗ %s: %v\n", s.description, err)
			return err
		}
		fmt.Fprintf(p.out, "✓ %s\n", s.description)
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

// stepsToRun returns the steps to provision the team that are not done yet.
func stepsToRun(team common.Team, status Status) []step {
	repo := team.GetRepoName()
	var steps []step
	if !status.RepoExists {
		steps = append(steps, step{
			description: fmt.Sprintf("Create the private repository %s/%s from the template %s", github.Org, repo, github.TemplateRepo),
			command:     github.CreateRepoCommand(repo),
		})
	}
	if !status.TeamExists {
		steps = append(steps, step{
			description: fmt.Sprintf("Create the secret GitHub team %s", team.Name),
			command:     github.CreateTeamCommand(team.Name),
		})
	}
	if status.RepoRole != "write" {
		steps = append(steps, step{
			description: fmt.Sprintf("Grant the GitHub team %s push access to %s", team.Name, repo),
			command:     github.GrantPushCommand(team.Name, repo),
		})
	}
	for _, member := range team.Members {
		if state := status.States[member.Github]; state == "active" || state == "pending" {
			continue
		}
		steps = append(steps, step{
			description: fmt.Sprintf("Add @%s to the GitHub team %s, which invites them to the organization", member.Github, team.Name),
			command:     github.AddMemberCommand(team.Name, member.Github),
		})
	}
	return steps
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
