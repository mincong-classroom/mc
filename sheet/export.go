package sheet

import (
	"fmt"
	"slices"
	"sort"
)

// NewExport returns an empty file of the last export, for the tab of the file f.
func NewExport(f *File) *File {
	columns := make(map[string]string, len(f.Columns))
	for letter, header := range f.Columns {
		columns[letter] = header
	}
	return &File{Tab: f.Tab, Keys: slices.Clone(f.Keys), Columns: columns}
}

// RecordPasted moves the cells of the blocks that were pasted from pending, the file of what
// waits, to exported, the file of the last export: the cells of the other blocks wait still. A
// row of the export takes the number of the row of pending, where its cells were pasted. A new
// row stays new until its keys are pasted: they stay in pending, since they find the row. It
// returns the number of cells moved.
func RecordPasted(pending, exported *File, pasted []Block) (int, error) {
	if err := sameLayout(pending, exported); err != nil {
		return 0, err
	}
	for letter, header := range pending.Columns {
		exported.Columns[letter] = header
	}
	moved := map[cell]bool{}
	for _, b := range pasted {
		for row := b.FirstRow; row <= b.LastRow; row++ {
			for column := b.FirstColumn; column <= b.LastColumn; column++ {
				moved[cell{row: row, column: column}] = true
			}
		}
	}
	columns := pending.columnOf()
	keys := map[string]bool{}
	for _, k := range pending.Keys {
		keys[k] = true
	}
	exportedRows := map[string]int{}
	for i, row := range exported.Rows {
		key, _ := exported.keyOf(row)
		exportedRows[key] = i
	}
	count := 0
	var waiting []Row
	for _, row := range pending.Rows {
		key, _ := pending.keyOf(row)
		i, ok := exportedRows[key]
		if !ok {
			i = len(exported.Rows)
			exportedRows[key] = i
			r := Row{Cells: map[string]any{}}
			for _, k := range pending.Keys {
				r.Cells[k] = row.Cells[k]
			}
			exported.Rows = append(exported.Rows, r)
		}
		left, keysLeft := 0, 0
		for _, header := range sortedKeys(row.Cells) {
			pasted := moved[cell{row: row.Number, column: columns[header]}]
			if keys[header] {
				if row.New && !pasted {
					keysLeft++
				}
				continue
			}
			if !pasted {
				left++
				continue
			}
			exported.Rows[i].Cells[header] = row.Cells[header]
			exported.Rows[i].Number = row.Number
			delete(row.Cells, header)
			delete(row.Check, header)
			count++
		}
		if row.New && keysLeft == 0 {
			row.New = false
			exported.Rows[i].Number = row.Number
		}
		if left+keysLeft > 0 {
			waiting = append(waiting, row)
		}
	}
	pending.Rows = waiting
	// A row of the export that no paste reached yet, such as a row of a new team, has no number:
	// it can't be checked against the tab.
	rows := exported.Rows[:0]
	for _, row := range exported.Rows {
		if row.Number > 0 {
			rows = append(rows, row)
		}
	}
	exported.Rows = rows
	sort.SliceStable(exported.Rows, func(i, j int) bool { return exported.Rows[i].Number < exported.Rows[j].Number })
	return count, nil
}

// sameLayout returns an error if two files of a tab disagree on their tab, their keys or the
// header of a column: the layout of the tab changed between them.
func sameLayout(a, b *File) error {
	if a.Tab != b.Tab {
		return fmt.Errorf("the tab %q is not the tab %q", a.Tab, b.Tab)
	}
	if !slices.Equal(a.Keys, b.Keys) {
		return fmt.Errorf("the keys %q are not the keys %q", a.Keys, b.Keys)
	}
	for _, letter := range a.letters() {
		if header, ok := b.Columns[letter]; ok && header != a.Columns[letter] {
			return fmt.Errorf("the column %s is %q in one file and %q in the other: the layout of the tab changed", letter, a.Columns[letter], header)
		}
	}
	return nil
}

// Verify compares the tab, as read, with the last export, and returns each difference: a header
// that changed, a row that moved or that the tab no longer has, and a cell whose value is no
// longer the one written. A formula is never compared, since the tab gives its result.
func Verify(read, exported *File) ([]string, error) {
	if read.Tab != exported.Tab || !slices.Equal(read.Keys, exported.Keys) {
		return nil, fmt.Errorf("the tab read, %q with the keys %q, is not the one of the export, %q with the keys %q", read.Tab, read.Keys, exported.Tab, exported.Keys)
	}
	var differences []string
	for _, letter := range exported.letters() {
		want := exported.Columns[letter]
		switch got, ok := read.Columns[letter]; {
		case !ok:
			differences = append(differences, fmt.Sprintf("column %s: not read, whereas the export writes %q", letter, want))
		case got != want:
			differences = append(differences, fmt.Sprintf("column %s: %q in the tab, %q at the export", letter, got, want))
		}
	}
	readRows := map[string]Row{}
	for _, row := range read.Rows {
		key, _ := read.keyOf(row)
		readRows[key] = row
	}
	columns := exported.columnOf()
	keys := map[string]bool{}
	for _, k := range exported.Keys {
		keys[k] = true
	}
	// A cell is compared with the one at the same place, so that a header changed in the tab is
	// one difference, not one per row.
	readHeaders := map[string]string{} // The header in the tab of each header of the export.
	for letter, header := range exported.Columns {
		if h, ok := read.Columns[letter]; ok {
			readHeaders[header] = h
		}
	}
	for _, row := range exported.Rows {
		key, _ := exported.keyOf(row)
		name := exported.describe(row)
		got, ok := readRows[key]
		if !ok {
			differences = append(differences, fmt.Sprintf("%s: no row of the tab has these keys", name))
			continue
		}
		if got.Number != row.Number {
			differences = append(differences, fmt.Sprintf("%s: row %d in the tab, row %d at the export", name, got.Number, row.Number))
		}
		for _, header := range exported.headersOf(row.Cells) {
			readHeader, ok := readHeaders[header]
			if keys[header] || !ok {
				continue // A column not read is a difference of its own, above.
			}
			if value := got.Cells[readHeader]; !SameValue(value, row.Cells[header]) {
				differences = append(differences, fmt.Sprintf("%s!%s %s, %s: %s in the tab, %s at the export",
					tabName(exported.Tab), a1(got.Number, columns[header]), name, header, text(value), text(row.Cells[header])))
			}
		}
	}
	return differences, nil
}
