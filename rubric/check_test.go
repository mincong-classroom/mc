package rubric

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// minimal is a valid file of rules, which each case of TestCheck breaks in one place.
const minimal = `title: Evaluation Rules
labs:
  L1:
    title: Build an Image
    report: docs/lab-1.md
    reference: answers/lab-1.md
    deadline: "2030-03-04T08:30:00+01:00"
  L2:
    title: Run a Pod
    report: docs/lab-2.md
    reference: answers/lab-2.md
    deadline: "2030-03-11T08:30:00+01:00"
scoring:
  late_penalty:
    min: 0.1
    max: 0.2
  penalties:
    secret:
      remove: 0.3
      when: The repository exposes a token.
evidence:
  kinds:
    file: A file.
    output: An output.
analysis_levels:
  none: Commands only.
rules:
  L1_BLD:
    lab: L1
    name: Build Test
    exercise: "1"
    requirement: mandatory
    coefficient: 0.6
    description: The team writes a Dockerfile.
    analysis:
      level: none
    evidence:
      asked:
        - file: the Dockerfile
    files:
      team:
        - app/Dockerfile
      teacher:
        - answers/Dockerfile
  L1_TAG:
    lab: L1
    name: Publish Test
    exercise: "2"
    requirement: mandatory
    depends_on: [L1_BLD]
    coefficient: 0.4
    description: The team publishes the image.
    analysis:
      level: none
    evidence:
      asked:
        - output: the push
    images:
      - sample/app-{team}:1.0.0
  L2_POD:
    lab: L2
    name: Pod Test
    exercise: "1"
    requirement: mandatory
    depends_on: [L1_TAG]
    coefficient: 1
    description: The team runs a Pod.
    analysis:
      level: none
    evidence:
      asked:
        - output: the logs
`

func TestCheck(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"answers/lab-1.md", "answers/lab-2.md", "answers/Dockerfile"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct {
		name     string
		old, new string   // The change of the file; old is empty for the valid file.
		want     []string // The problems, none for the valid file.
	}{
		{name: "valid"},
		{
			name: "a line too long",
			old:  "    description: The team writes a Dockerfile.",
			new:  "    description: The team writes a Dockerfile." + strings.Repeat(" More.", 15),
			want: []string{"34: the line has 136 characters, more than 120"},
		},
		{
			name: "a deadline without its offset",
			old:  `deadline: "2030-03-04T08:30:00+01:00"`,
			new:  `deadline: "2030-03-04 08:30"`,
			want: []string{`7: L1: the deadline "2030-03-04 08:30" is not a time with its offset to UTC, such as 2026-10-23T08:30:00+02:00`},
		},
		{
			name: "a report outside of the repository of the team",
			old:  "report: docs/lab-2.md",
			new:  "report: ../lab-2.md",
			want: []string{`10: L2: the path "../lab-2.md" is not relative to the root of the repository of the team`},
		},
		{
			name: "a missing reference",
			old:  "reference: answers/lab-2.md",
			new:  "reference: answers/lab-9.md",
			want: []string{`11: L2: the file "answers/lab-9.md" does not exist`},
		},
		{
			name: "a lab without rules",
			old:  "    deadline: \"2030-03-11T08:30:00+01:00\"\n",
			new:  "    deadline: \"2030-03-11T08:30:00+01:00\"\n  L3:\n    title: More\n    report: docs/lab-3.md\n    reference: answers/lab-2.md\n    deadline: \"2030-03-18T08:30:00+01:00\"\n",
			want: []string{"13: L3: no rule has the lab L3"},
		},
		{
			name: "a rule of a lab that doesn't exist",
			old:  "    lab: L2\n",
			new:  "    lab: L3\n",
			want: []string{"8: L2: no rule has the lab L2", `61: L2_POD: the lab "L3" is not a key of labs`},
		},
		{
			name: "an ID that doesn't start with its lab",
			old:  "  L1_TAG:\n",
			new:  "  L2_TAG:\n",
			want: []string{"46: L2_TAG: the ID does not start with its lab L1", "65: L2_POD: depends on L1_TAG, which is not a rule"},
		},
		{
			name: "an ID of 2 letters",
			old:  "  L1_TAG:\n",
			new:  "  L1_TG:\n",
			want: []string{"45: L1_TG: the ID of a rule is its lab, _ and 3 capital letters, such as L1_DKF", "65: L2_POD: depends on L1_TAG, which is not a rule"},
		},
		{
			name: "an exercise that is not a number",
			old:  `exercise: "2"`,
			new:  `exercise: "2a"`,
			want: []string{`48: L1_TAG: the exercise "2a" is not a number such as 4, 4.1 or 4.1.1`},
		},
		{
			name: "the exercises out of order",
			old:  `exercise: "2"`,
			new:  `exercise: "0.5"`,
			want: []string{"48: L1_TAG: the exercise 0.5 comes after 1, in the order of the file"},
		},
		{
			name: "an unknown requirement",
			old:  "    requirement: mandatory\n    depends_on: [L1_TAG]",
			new:  "    requirement: required\n    depends_on: [L1_TAG]",
			want: []string{`64: L2_POD: the requirement "required" is not mandatory or optional`},
		},
		{
			name: "a coefficient between 2 steps",
			old:  "coefficient: 0.4",
			new:  "coefficient: 0.42",
			want: []string{"3: L1: the coefficients of the mandatory rules add up to 1.02, not 1", "51: L1_TAG: the coefficient 0.42 does not go by steps of 0.05"},
		},
		{
			name: "the mandatory rules don't add up to 1",
			old:  "coefficient: 0.4",
			new:  "coefficient: 0.35",
			want: []string{"3: L1: the coefficients of the mandatory rules add up to 0.95, not 1"},
		},
		{
			name: "a penalty without its condition",
			old:  "      when: The repository exposes a token.\n",
			new:  "",
			want: []string{"18: a penalty says when it applies"},
		},
		{
			name: "an unknown level of analysis",
			old:  "      level: none\n    evidence:\n      asked:\n        - output: the push",
			new:  "      level: deep\n    evidence:\n      asked:\n        - output: the push",
			want: []string{`53: L1_TAG: the analysis level "deep" is not a key of analysis_levels`},
		},
		{
			name: "an unknown kind of evidence",
			old:  "- output: the push",
			new:  "- trace: the push",
			want: []string{`57: L1_TAG: the kind of evidence "trace" is not a key of evidence.kinds`},
		},
		{
			name: "a missing file of the teacher",
			old:  "- answers/Dockerfile",
			new:  "- answers/Containerfile",
			want: []string{`40: L1_BLD: the file "answers/Containerfile" does not exist`},
		},
		{
			name: "an image without a tag",
			old:  "- sample/app-{team}:1.0.0",
			new:  "- sample/app-{team}",
			want: []string{`58: L1_TAG: the image "sample/app-{team}" is not a name and a tag, such as namespace/app-{team}:1.0.0`},
		},
		{
			name: "a dependency that doesn't exist",
			old:  "depends_on: [L1_BLD]",
			new:  "depends_on: [L1_BLX]",
			want: []string{"50: L1_TAG: depends on L1_BLX, which is not a rule"},
		},
		{
			name: "a dependency of a later lab",
			old:  "depends_on: [L1_BLD]",
			new:  "depends_on: [L2_POD]",
			want: []string{"50: L1_TAG: depends on L2_POD, of a later lab", "50: L1_TAG: a cycle of dependencies: L1_TAG -> L2_POD -> L1_TAG"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := minimal
			if c.old != "" {
				if strings.Count(data, c.old) != 1 {
					t.Fatalf("the change %q matches %d times, want 1", c.old, strings.Count(data, c.old))
				}
				data = strings.Replace(data, c.old, c.new, 1)
			}
			r, err := Parse([]byte(data))
			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}

			problems := r.Check(CheckOptions{Root: root, MaxWidth: 120})

			var got []string
			for _, problem := range problems {
				got = append(got, problem.String())
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("Check() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCheckFindsACycle(t *testing.T) {
	data := strings.Replace(minimal, "    lab: L1\n    name: Build Test\n", "    lab: L1\n    depends_on: [L1_TAG]\n    name: Build Test\n", 1)
	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}

	problems := r.Check(CheckOptions{})

	if len(problems) != 1 || !strings.Contains(problems[0].Message, "a cycle of dependencies: L1_BLD -> L1_TAG -> L1_BLD") {
		t.Errorf("Check() = %v, want the cycle L1_BLD -> L1_TAG -> L1_BLD", problems)
	}
}
