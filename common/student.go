package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StudentList is the school list of a year: the students known before the course starts.
type StudentList struct {
	Students []Student
}

// Student is a student of the school list. Other keys, such as an email, are ignored.
type Student struct {
	Name string // Full name in format "LAST, First", as in the team registry
}

// StudentListPath returns the path of the school list of the current year.
func StudentListPath() string {
	return filepath.Join(ConfigDir(), fmt.Sprintf("students-%d.yaml", Year()))
}

// ListStudents reads the school list of the current year. The error wraps fs.ErrNotExist when
// the file does not exist.
func ListStudents() ([]Student, error) {
	data, err := os.ReadFile(StudentListPath())
	if err != nil {
		return nil, err
	}
	return ParseStudents(data)
}

// ParseStudents parses a school list. The result is never nil.
func ParseStudents(data []byte) ([]Student, error) {
	var list StudentList
	if err := yaml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the school list: %v", err)
	}
	if list.Students == nil {
		return []Student{}, nil
	}
	return list.Students, nil
}

// SameName reports whether two names in the format "LAST, First" designate the same person,
// ignoring the case and the extra spaces.
func SameName(a, b string) bool {
	return normalizeName(a) == normalizeName(b)
}

func normalizeName(name string) string {
	parts := strings.Split(name, ",")
	for i, part := range parts {
		parts[i] = strings.Join(strings.Fields(part), " ")
	}
	return strings.ToLower(strings.Join(parts, ", "))
}
