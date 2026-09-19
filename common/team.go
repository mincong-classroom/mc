package common

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultYear = 2026

// Year returns the year of the cohort, which selects the files read in ~/.mc. It can be
// overridden with the environment variable MC_YEAR, e.g. MC_YEAR=2025.
func Year() int {
	if year, err := strconv.Atoi(os.Getenv("MC_YEAR")); err == nil {
		return year
	}
	return defaultYear
}

// ConfigDir returns the directory holding the private data of the classroom, i.e. ~/.mc.
func ConfigDir() string {
	return filepath.Join(os.Getenv("HOME"), ".mc")
}

// TeamRegistryFile replaces the team registry of the current year when it is not empty, e.g. to
// try the commands on a test registry. It is set by the global flag --team-file.
var TeamRegistryFile string

// TeamRegistryPath returns the path of the team registry: TeamRegistryFile if set, otherwise the
// registry of the current year.
func TeamRegistryPath() string {
	if TeamRegistryFile == "" {
		return filepath.Join(ConfigDir(), fmt.Sprintf("teams-%d.yaml", Year()))
	}
	// The shell does not expand the "~" of --team-file=~/...
	if rest, ok := strings.CutPrefix(TeamRegistryFile, "~/"); ok {
		return filepath.Join(os.Getenv("HOME"), rest)
	}
	return TeamRegistryFile
}

// LoadRegistry reads the team registry.
func LoadRegistry() (*TeamRegistry, error) {
	teamData, err := os.ReadFile(TeamRegistryPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var data TeamRegistry
	err = yaml.Unmarshal(teamData, &data)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}
	// A typo in a top-level key, such as "student", would silently lose its content.
	var keys map[string]any
	if err := yaml.Unmarshal(teamData, &keys); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}
	var unknown []string
	for key := range keys {
		if key != "students" && key != "teams" {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown key %q in %s: expected \"students\" or \"teams\"", unknown[0], TeamRegistryPath())
	}
	return &data, nil
}

// ListTeams returns the teams of the team registry.
func ListTeams() ([]Team, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return registry.Teams, nil
}

// FilterTeams returns the teams with the given names, or an error if a name is not registered.
func FilterTeams(teams []Team, names []string) ([]Team, error) {
	var selected []Team
	for _, name := range names {
		i := slices.IndexFunc(teams, func(team Team) bool { return team.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("team %q not found in %s", name, TeamRegistryPath())
		}
		selected = append(selected, teams[i])
	}
	return selected, nil
}

// memberYAML is a team member as written in the registry by AddTeam and SetTeamMembers.
type memberYAML struct {
	Name   string `yaml:"name"`
	Github string `yaml:"github"`
}

// AddTeam appends the team to the team registry file, keeping its comments.
func AddTeam(team Team) error {
	return editRegistry(func(root *yaml.Node) error {
		teams := mappingValue(root, "teams")
		if teams.Kind != yaml.SequenceNode { // "teams:" without value
			*teams = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", HeadComment: teams.HeadComment, LineComment: teams.LineComment}
		}
		teams.Style = 0 // "teams: []" becomes a block sequence

		members, err := membersNode(team.Members)
		if err != nil {
			return err
		}
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "name"}, {Kind: yaml.ScalarNode, Tag: "!!str", Value: team.Name},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "members"}, members,
		}}
		teams.Content = append(teams.Content, node)
		return nil
	})
}

// SetTeamMembers replaces the members of the team in the team registry file, keeping its comments.
func SetTeamMembers(name string, members []TeamMember) error {
	return editRegistry(func(root *yaml.Node) error {
		if teams := findValue(root, "teams"); teams != nil {
			for _, team := range teams.Content {
				if value := findValue(team, "name"); value == nil || value.Value != name {
					continue
				}
				node, err := membersNode(members)
				if err != nil {
					return err
				}
				*mappingValue(team, "members") = *node
				return nil
			}
		}
		return fmt.Errorf("team %q not found", name)
	})
}

// editRegistry applies the edit to the root mapping of the team registry file. The comments are
// kept, and the file is written to a new file then renamed, so that it is never left half written.
func editRegistry(edit func(root *yaml.Node) error) error {
	path := TeamRegistryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to unmarshal data: %v", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s is not a mapping", path)
	}
	if err := edit(doc.Content[0]); err != nil {
		return err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// membersNode returns the YAML sequence of the members, with their names quoted, as they are
// written by hand.
func membersNode(members []TeamMember) (*yaml.Node, error) {
	values := []memberYAML{}
	for _, member := range members {
		values = append(values, memberYAML(member))
	}
	var node yaml.Node
	if err := node.Encode(values); err != nil {
		return nil, err
	}
	for _, member := range node.Content {
		if name := findValue(member, "name"); name != nil {
			name.Style = yaml.DoubleQuotedStyle
		}
	}
	return &node, nil
}

// findValue returns the value of the key in the mapping, or nil.
func findValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// mappingValue returns the value of the key in the mapping, added as an empty sequence when missing.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if value := findValue(mapping, key); value != nil {
		return value
	}
	value := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	return value
}
