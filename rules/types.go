package rules

type Rule interface {
	// Id The ID of the rule. The combination of Symbol and LabId, such as "L1_JAR", "L1_MST", etc.
	Id() string

	// Symbol The symbol of the rule, such as "JAR", "MST", etc.
	Symbol() string

	// LabId The ID of the lab session of this rule, such as "L1", "L2", "L3", etc.
	LabId() string

	// Run Run the rule for the given team.
	Run(team string) error
}

type TeamAssignmentL1 struct {
	MavenCommand string `yaml:"mvn_command"`
}
