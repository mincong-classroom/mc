// Package sheet keeps the data of the tabs of a spreadsheet in JSON files, and moves it between
// the files and the spreadsheet by hand or through a browser: the blocks of cells to paste, the
// script that reads a tab, and the check of a tab against what was written.
//
// A file holds one tab: one object per row, like the documents of a collection in MongoDB, whose
// attributes are the columns, named by their headers. A row holds the keys that find it, and the
// cells to write only: an attribute absent is a cell left alone, never cleared. A new row, which
// the tab doesn't have yet, gets its keys written too.
package sheet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is the data of one tab of a spreadsheet.
type File struct {
	// Tab is the name of the tab, such as "Evaluation".
	Tab string
	// Keys are the headers whose values find a row, such as Team, Lab and Rule.
	Keys []string
	// Columns are the headers of the columns that the file knows, by their letters, such as "G":
	// "Completeness". A column out of it is never read nor written, such as a column of personal
	// data.
	Columns map[string]string
	// Rows are the rows of the tab, in the order of the file.
	Rows []Row
}

// Row is a row of a tab.
type Row struct {
	// Number is the number of the row in the tab, the attribute "_row": 2 for the first row under
	// the headers.
	Number int
	// Cells are the values of the row by header, the keys included: a string, a json.Number or a
	// bool. A string that starts with "=" is a formula.
	Cells map[string]any
	// Check says why the teacher should look at a cell before it is written, by header, the
	// attribute "_check": a cell that the teacher may have changed since the last export.
	Check map[string]string
	// New says that the tab doesn't have the row yet, the attribute "_new": its keys are written
	// with its cells, at its number.
	New bool
}

// The attributes of a row that are not columns.
const (
	rowAttribute   = "_row"
	checkAttribute = "_check"
	newAttribute   = "_new"
)

// Load reads a file and checks it: every problem is in the error, one per line.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if problems := f.Problems(); len(problems) > 0 {
		errs := make([]error, len(problems))
		for i, p := range problems {
			errs[i] = fmt.Errorf("%s: %s", path, p)
		}
		return nil, errors.Join(errs...)
	}
	return &f, nil
}

// Save writes the file, readable by its owner only: it may hold an evaluation. The attributes of
// each row follow the order of the columns.
func (f *File) Save(path string) error {
	raw, err := f.MarshalJSON() // Not json.Marshal, which escapes <, > and & again.
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return err
	}
	out.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // Fails once renamed: nothing to remove then.
	if _, err := tmp.Write(out.Bytes()); err != nil {
		_ = tmp.Close() // The error of the write says more.
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Problems returns what is wrong in the file, one problem per item.
func (f *File) Problems() []string {
	var problems []string
	if f.Tab == "" {
		problems = append(problems, `no "tab"`)
	}
	headers := map[string]string{} // The letters of each header.
	for _, letter := range f.letters() {
		header := f.Columns[letter]
		if _, err := columnIndex(letter); err != nil {
			problems = append(problems, fmt.Sprintf("columns: %v", err))
		}
		switch {
		case header == "" || strings.HasPrefix(header, "_"):
			problems = append(problems, fmt.Sprintf("columns: the header %q of the column %s doesn't name a column", header, letter))
		case headers[header] != "":
			problems = append(problems, fmt.Sprintf("columns: the header %q is the one of %s and %s", header, headers[header], letter))
		default:
			headers[header] = letter
		}
	}
	if len(f.Keys) == 0 {
		problems = append(problems, `no "keys": the headers whose values find a row`)
	}
	for _, key := range f.Keys {
		if headers[key] == "" {
			problems = append(problems, fmt.Sprintf("keys: %q is not a header of the columns", key))
		}
	}
	numbers := map[int]int{}
	keys := map[string]int{}
	for i, row := range f.Rows {
		where := fmt.Sprintf("rows[%d] (row %d)", i, row.Number)
		if row.Number < 2 {
			problems = append(problems, fmt.Sprintf("rows[%d]: the attribute %s is %d, whereas the first row under the headers is 2", i, rowAttribute, row.Number))
		} else if j, ok := numbers[row.Number]; ok {
			problems = append(problems, fmt.Sprintf("%s: the row of rows[%d] too", where, j))
		} else {
			numbers[row.Number] = i
		}
		for _, header := range sortedKeys(row.Cells) {
			if headers[header] == "" {
				problems = append(problems, fmt.Sprintf("%s: %q is not a header of the columns", where, header))
				continue
			}
			if err := checkValue(row.Cells[header]); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %s: %v", where, header, err))
			}
		}
		for _, header := range sortedKeys(row.Check) {
			if _, ok := row.Cells[header]; !ok {
				problems = append(problems, fmt.Sprintf("%s: %s: %q is not a cell of the row", where, checkAttribute, header))
			}
		}
		key, missing := f.keyOf(row)
		for _, k := range missing {
			problems = append(problems, fmt.Sprintf("%s: no key %q, a text", where, k))
		}
		if len(missing) > 0 {
			continue
		}
		if j, ok := keys[key]; ok {
			problems = append(problems, fmt.Sprintf("%s: the keys of rows[%d] too, %s", where, j, f.describe(row)))
		} else {
			keys[key] = i
		}
	}
	return problems
}

// checkValue returns an error if v is not a value of a cell: a non-empty text, a number or a
// boolean.
func checkValue(v any) error {
	switch v := v.(type) {
	case string:
		if v == "" {
			return errors.New(`an empty text "" would clear the cell: leave the attribute out to leave the cell alone`)
		}
	case json.Number, bool:
	default:
		return fmt.Errorf("%v is not a text, a number or a boolean", v)
	}
	return nil
}

// keyOf returns the values of the keys of the row, and the keys that the row misses.
func (f *File) keyOf(row Row) (key string, missing []string) {
	values := make([]string, len(f.Keys))
	for i, k := range f.Keys {
		s, ok := row.Cells[k].(string)
		if !ok || s == "" {
			missing = append(missing, k)
		}
		values[i] = s
	}
	return strings.Join(values, "\x1f"), missing
}

// describe returns the values of the keys of the row, to name it in a message: "teal L2 L2_RUN".
func (f *File) describe(row Row) string {
	values := make([]string, len(f.Keys))
	for i, k := range f.Keys {
		values[i] = fmt.Sprint(row.Cells[k])
	}
	return strings.Join(values, " ")
}

// letters returns the letters of the columns, from left to right.
func (f *File) letters() []string {
	letters := make([]string, 0, len(f.Columns))
	for letter := range f.Columns {
		letters = append(letters, letter)
	}
	sort.Slice(letters, func(i, j int) bool {
		a, b := letters[i], letters[j]
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
	return letters
}

// columnOf returns the index of the column of each header.
func (f *File) columnOf() map[string]int {
	indexes := make(map[string]int, len(f.Columns))
	for letter, header := range f.Columns {
		if index, err := columnIndex(letter); err == nil {
			indexes[header] = index
		}
	}
	return indexes
}

// headersOf returns the headers of the map m in the order of the columns.
func (f *File) headersOf(m map[string]any) []string {
	columns := f.columnOf()
	headers := sortedKeys(m)
	sort.SliceStable(headers, func(i, j int) bool { return columns[headers[i]] < columns[headers[j]] })
	return headers
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// fileJSON is the layout of a file, whose rows are decoded by hand: their attributes are the
// headers of the columns.
type fileJSON struct {
	Tab     string                       `json:"tab"`
	Keys    []string                     `json:"keys"`
	Columns map[string]string            `json:"columns"`
	Rows    []map[string]json.RawMessage `json:"rows"`
}

// UnmarshalJSON decodes a file. An attribute of a row that starts with "_" and that the file
// doesn't know, such as "_rwo", is an error, so that a typo doesn't go unnoticed.
func (f *File) UnmarshalJSON(data []byte) error {
	var raw fileJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	*f = File{Tab: raw.Tab, Keys: raw.Keys, Columns: raw.Columns}
	for i, attributes := range raw.Rows {
		row := Row{Cells: map[string]any{}}
		for _, name := range sortedKeys(attributes) {
			value := attributes[name]
			var err error
			switch {
			case name == rowAttribute:
				err = json.Unmarshal(value, &row.Number)
			case name == checkAttribute:
				err = json.Unmarshal(value, &row.Check)
			case name == newAttribute:
				err = json.Unmarshal(value, &row.New)
			case strings.HasPrefix(name, "_"):
				err = fmt.Errorf("unknown attribute: only %s, %s and %s start with _", rowAttribute, newAttribute, checkAttribute)
			default:
				d := json.NewDecoder(bytes.NewReader(value))
				d.UseNumber()
				var v any
				err = d.Decode(&v)
				row.Cells[name] = v
			}
			if err != nil {
				return fmt.Errorf("rows[%d]: %s: %w", i, name, err)
			}
		}
		if _, ok := attributes[rowAttribute]; !ok {
			return fmt.Errorf("rows[%d]: no %s, the number of the row in the tab", i, rowAttribute)
		}
		f.Rows = append(f.Rows, row)
	}
	return nil
}

// MarshalJSON encodes a file: the columns from left to right, and the attributes of each row in
// the order of the columns, after "_row" and "_new", and before "_check".
func (f *File) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`{"tab":`)
	writeJSON(&b, f.Tab)
	b.WriteString(`,"keys":`)
	writeJSON(&b, f.Keys)
	b.WriteString(`,"columns":{`)
	for i, letter := range f.letters() {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSON(&b, letter)
		b.WriteByte(':')
		writeJSON(&b, f.Columns[letter])
	}
	b.WriteString(`},"rows":[`)
	for i, row := range f.Rows {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"` + rowAttribute + `":`)
		writeJSON(&b, row.Number)
		if row.New {
			b.WriteString(`,"` + newAttribute + `":true`)
		}
		for _, header := range f.headersOf(row.Cells) {
			b.WriteByte(',')
			writeJSON(&b, header)
			b.WriteByte(':')
			writeJSON(&b, row.Cells[header])
		}
		if len(row.Check) > 0 {
			b.WriteString(`,"` + checkAttribute + `":{`)
			check := make(map[string]any, len(row.Check))
			for header, why := range row.Check {
				check[header] = why
			}
			for j, header := range f.headersOf(check) {
				if j > 0 {
					b.WriteByte(',')
				}
				writeJSON(&b, header)
				b.WriteByte(':')
				writeJSON(&b, row.Check[header])
			}
			b.WriteByte('}')
		}
		b.WriteByte('}')
	}
	b.WriteString(`]}`)
	return b.Bytes(), nil
}

// writeJSON writes v in JSON, without escaping <, > and &, which a comment may hold.
func writeJSON(b *bytes.Buffer, v any) {
	e := json.NewEncoder(b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)         // Strings, numbers, booleans and slices of strings: it never fails.
	b.Truncate(b.Len() - 1) // The newline of Encode.
}
