package sheet

import (
	"fmt"
	"regexp"
)

var letters = regexp.MustCompile(`^[A-Z]{1,3}$`)

// columnIndex returns the index of a column from its letters, from 0 for A: 25 for Z, 26 for AA.
func columnIndex(column string) (int, error) {
	if !letters.MatchString(column) {
		return 0, fmt.Errorf("%q is not a column, such as A or AB", column)
	}
	index := 0
	for _, c := range column {
		index = index*26 + int(c-'A') + 1
	}
	return index - 1, nil
}
