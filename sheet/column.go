package sheet

import (
	"fmt"
	"regexp"
	"strconv"
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

// columnLetters returns the letters of the column of an index, from 0 for A.
func columnLetters(index int) string {
	var b []byte
	for index++; index > 0; index = (index - 1) / 26 {
		b = append([]byte{byte('A' + (index-1)%26)}, b...)
	}
	return string(b)
}

// a1 returns the A1 notation of the cell at the row and the index of a column, such as G99.
func a1(row, column int) string {
	return columnLetters(column) + strconv.Itoa(row)
}
