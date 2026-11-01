package sheet

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Block is a rectangle of adjacent cells to write, every one of them: a paste of a block never
// clears a cell, whereas a paste of an empty cell would.
type Block struct {
	// Tab is the name of the tab.
	Tab string
	// FirstRow and LastRow are the numbers of its first and last rows.
	FirstRow, LastRow int
	// FirstColumn and LastColumn are the indexes of its first and last columns, from 0 for A.
	FirstColumn, LastColumn int
	// Texts are the texts to paste, row by row, as Sheets reads them when they are typed.
	Texts [][]string
	// Check says why the teacher should look at its cells before pasting them, one line per cell,
	// such as "G99 Completeness: 0.8 at the last export". Empty for most blocks.
	Check []string
}

// Range returns the range of the block in A1 notation, with its tab: Evaluation!N80:Q157.
func (b Block) Range() string {
	r := tabName(b.Tab) + "!" + a1(b.FirstRow, b.FirstColumn)
	if b.FirstRow != b.LastRow || b.FirstColumn != b.LastColumn {
		r += ":" + a1(b.LastRow, b.LastColumn)
	}
	return r
}

// Size returns the rows and the columns of the block, such as "78 × 4".
func (b Block) Size() string {
	return fmt.Sprintf("%d × %d", b.LastRow-b.FirstRow+1, b.LastColumn-b.FirstColumn+1)
}

// TSV returns the texts of the block as tab-separated values, which a paste spreads on its cells.
func (b Block) TSV() string {
	var s strings.Builder
	for _, row := range b.Texts {
		s.WriteString(strings.Join(row, "\t"))
		s.WriteByte('\n')
	}
	return s.String()
}

var plainTabName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// tabName returns the name of a tab as a range writes it: 'Scores Final' within quotes.
func tabName(tab string) string {
	if plainTabName.MatchString(tab) {
		return tab
	}
	return "'" + strings.ReplaceAll(tab, "'", "''") + "'"
}

// cell is the place of a cell in a tab.
type cell struct {
	row, column int
}

// Blocks returns the blocks to paste to write every cell of the file but its keys, which find the
// rows and are in the tab already, except in a new row: from left to right, then from top to
// bottom. A cell with a "_check" is in a block of its own, after the others, so that the teacher
// can look at it first.
func Blocks(f *File) ([]Block, error) {
	columns := f.columnOf()
	keys := map[string]bool{}
	for _, k := range f.Keys {
		keys[k] = true
	}
	texts := map[cell]string{}
	checks := map[cell]string{}
	var errs []string
	for _, row := range f.Rows {
		for header, value := range row.Cells {
			if keys[header] && !row.New {
				continue
			}
			c := cell{row: row.Number, column: columns[header]}
			text, err := pasteText(value)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s %s: %v", a1(c.row, c.column), header, err))
				continue
			}
			texts[c] = text
			if why, ok := row.Check[header]; ok {
				checks[c] = fmt.Sprintf("%s %s: %s", a1(c.row, c.column), header, why)
			}
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	plain := map[cell]bool{}
	checked := map[cell]bool{}
	for c := range texts {
		if _, ok := checks[c]; ok {
			checked[c] = true
		} else {
			plain[c] = true
		}
	}
	var blocks []Block
	for _, group := range []map[cell]bool{plain, checked} {
		for _, r := range rectangles(group) {
			b := r
			b.Tab = f.Tab
			for row := b.FirstRow; row <= b.LastRow; row++ {
				line := make([]string, 0, b.LastColumn-b.FirstColumn+1)
				for column := b.FirstColumn; column <= b.LastColumn; column++ {
					c := cell{row: row, column: column}
					line = append(line, texts[c])
					if why, ok := checks[c]; ok {
						b.Check = append(b.Check, why)
					}
				}
				b.Texts = append(b.Texts, line)
			}
			blocks = append(blocks, b)
		}
	}
	return blocks, nil
}

// rectangles splits a set of cells into rectangles that cover each cell once: the runs of
// adjacent rows of each column, then those of the same rows in adjacent columns, merged.
func rectangles(cells map[cell]bool) []Block {
	rowsOf := map[int][]int{}
	for c := range cells {
		rowsOf[c.column] = append(rowsOf[c.column], c.row)
	}
	columns := make([]int, 0, len(rowsOf))
	for column := range rowsOf {
		columns = append(columns, column)
	}
	sort.Ints(columns)
	type run struct{ first, last int }
	var blocks []*Block
	open := map[run]*Block{} // The rectangles that end at the column before.
	for _, column := range columns {
		rows := rowsOf[column]
		sort.Ints(rows)
		next := map[run]*Block{}
		for i := 0; i < len(rows); {
			j := i
			for j+1 < len(rows) && rows[j+1] == rows[j]+1 {
				j++
			}
			r := run{first: rows[i], last: rows[j]}
			if b, ok := open[r]; ok && b.LastColumn == column-1 {
				b.LastColumn = column
				next[r] = b
			} else {
				b := &Block{FirstRow: r.first, LastRow: r.last, FirstColumn: column, LastColumn: column}
				blocks = append(blocks, b)
				next[r] = b
			}
			i = j + 1
		}
		open = next
	}
	out := make([]Block, len(blocks))
	for i, b := range blocks {
		out[i] = *b
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FirstColumn != out[j].FirstColumn {
			return out[i].FirstColumn < out[j].FirstColumn
		}
		return out[i].FirstRow < out[j].FirstRow
	})
	return out
}
