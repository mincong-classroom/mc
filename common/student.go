package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Student is a student of the school list, known before the course starts.
type Student struct {
	LastName  string
	FirstName string
}

// FullName returns the name in the format "LAST, First", as used in the team registry.
func (s Student) FullName() string {
	return fmt.Sprintf("%s, %s", s.LastName, s.FirstName)
}

// StudentListPath returns the path of the school list of the current year.
func StudentListPath() string {
	return filepath.Join(ConfigDir(), fmt.Sprintf("students-%d.tsv", Year()))
}

// ListStudents reads the school list of the current year. The error wraps fs.ErrNotExist when
// the file does not exist.
func ListStudents() ([]Student, error) {
	file, err := os.Open(StudentListPath())
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParseStudents(file)
}

// ParseStudents parses a school list: tab-separated, no header, one student per line, in the
// format "LAST<TAB>First". Other columns, such as an email, are ignored. The result is never nil.
func ParseStudents(r io.Reader) ([]Student, error) {
	students := []Student{}
	scanner := bufio.NewScanner(r)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("line %d: expected at least 2 tab-separated columns, got %d", lineNumber, len(fields))
		}
		students = append(students, Student{
			LastName:  strings.TrimSpace(fields[0]),
			FirstName: strings.TrimSpace(fields[1]),
		})
	}
	return students, scanner.Err()
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
