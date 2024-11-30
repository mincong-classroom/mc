package rules

// Rule represents a rule to grade the assignment.
//
// The type T is the type of the options to be used when executing the rule.
type Rule[T any] interface {
	// Id The ID of the rule. The combination of Symbol and LabId, such as "L1_JAR", "L1_MST", etc.
	Id() string

	// Symbol The symbol of the rule, such as "JAR", "MST", etc.
	Symbol() string

	// LabId The ID of the lab session of this rule, such as "L1", "L2", "L3", etc.
	LabId() string

	// Exercice The exercice of the rule, such as "1.1", "1.2", "2.1", etc.
	Exercice() string

	// Name The name of the rule.
	Name() string

	// Run Run the rule for the given team.
	Run(team string, opts T) error

	// Description The description of the rule.
	Description() string
}

type TeamAssignmentL1 struct {
	MavenCommand string `yaml:"mvn_command"`
}
