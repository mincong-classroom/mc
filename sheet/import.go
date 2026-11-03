package sheet

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Export is what mc writes in a spreadsheet: the range of mc of each tab, every row of it. The
// script of ImportScript imports it in the spreadsheet: it finds each row by its keys, then
// rewrites the range of mc, so that a cell that the export doesn't give becomes empty. The other
// columns, such as those of the teacher, are never written.
type Export struct {
	// Spreadsheet is the ID of the spreadsheet: the script refuses to import the export in another
	// one. Empty, it imports it anywhere.
	Spreadsheet string
	Tabs        []*Tab
}

// Tab is the range of mc in a tab of the spreadsheet.
type Tab struct {
	// Name is the name of the tab, such as "Evaluation".
	Name string
	// Keys are the headers whose values find a row, such as Team, Lab and Rule.
	Keys []string
	// Columns are the headers of the columns of mc by their letters, such as "G": "TA
	// Completeness": the keys, and the range of mc. The script checks that the first row of the tab
	// has them, and stops before any write otherwise.
	Columns map[string]string
	// Formulas are the columns whose formula is the same on every row, by header, such as "Score":
	// "=G{row}*F{row}", where {row} is the number of the row.
	Formulas map[string]string
	// Fill are the letters of the columns that a new row copies from the row above, like a fill
	// down, such as the formulas of the students in B and C, which mc never reads.
	Fill []string
	// Rows are the rows of the tab, in the order of the tab: a new key gets a new row at the end.
	// Their cells are by header: a text, a json.Number or a bool, and a text that starts with = is
	// a formula, such as a penalty, where {row} is the number of the row. A header absent is an
	// empty cell.
	Rows []map[string]any
}

// RowPlaceholder is the number of its row in a formula of an export, such as =G{row}*F{row}.
const RowPlaceholder = "{row}"

//go:embed import.gs
var importScript string

// ImportScript returns the source of the script that imports an export in the spreadsheet, with
// Apps Script: it adds the menu "mc", then "Import", to the spreadsheet.
func ImportScript() string {
	return importScript
}

// LoadExport reads an export and checks it: every problem is in the error, one per line.
func LoadExport(path string) (*Export, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e Export
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if problems := e.Problems(); len(problems) > 0 {
		errs := make([]error, len(problems))
		for i, p := range problems {
			errs[i] = fmt.Errorf("%s: %s", path, p)
		}
		return nil, errors.Join(errs...)
	}
	return &e, nil
}

// Save writes the export, readable by its owner only: it holds the evaluations. The script
// imports the same JSON.
func (e *Export) Save(path string) error {
	raw, err := e.MarshalJSON()
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

// Tab returns the tab of the name, nil if the export doesn't have it.
func (e *Export) Tab(name string) *Tab {
	for _, t := range e.Tabs {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// Problems returns what is wrong in the export, one problem per item.
func (e *Export) Problems() []string {
	var problems []string
	if len(e.Tabs) == 0 {
		problems = append(problems, `no "tabs"`)
	}
	names := map[string]bool{}
	for i, t := range e.Tabs {
		if names[t.Name] {
			problems = append(problems, fmt.Sprintf("tabs[%d]: the tab %q comes twice", i, t.Name))
		}
		names[t.Name] = true
		for _, p := range t.Problems() {
			problems = append(problems, fmt.Sprintf("tabs[%d] (%s): %s", i, t.Name, p))
		}
	}
	return problems
}

// Problems returns what is wrong in the tab, one problem per item.
func (t *Tab) Problems() []string {
	var problems []string
	if t.Name == "" {
		problems = append(problems, `no "name"`)
	}
	headers := map[string]string{} // The letters of each header.
	for _, letter := range t.Letters() {
		header := t.Columns[letter]
		if _, err := columnIndex(letter); err != nil {
			problems = append(problems, fmt.Sprintf("columns: %v", err))
		}
		switch {
		case header == "":
			problems = append(problems, fmt.Sprintf("columns: the column %s has no header", letter))
		case headers[header] != "":
			problems = append(problems, fmt.Sprintf("columns: the header %q is the one of %s and %s", header, headers[header], letter))
		default:
			headers[header] = letter
		}
	}
	if len(t.Keys) == 0 {
		problems = append(problems, `no "keys": the headers whose values find a row`)
	}
	keys := map[string]bool{}
	for _, k := range t.Keys {
		keys[k] = true
		if headers[k] == "" {
			problems = append(problems, fmt.Sprintf("keys: %q is not a header of the columns", k))
		}
		if _, ok := t.Formulas[k]; ok {
			problems = append(problems, fmt.Sprintf("formulas: %q is a key", k))
		}
	}
	for _, header := range sortedKeys(t.Formulas) {
		if headers[header] == "" {
			problems = append(problems, fmt.Sprintf("formulas: %q is not a header of the columns", header))
		}
		if !strings.HasPrefix(t.Formulas[header], "=") {
			problems = append(problems, fmt.Sprintf("formulas: %s: %q doesn't start with =", header, t.Formulas[header]))
		}
	}
	for _, letter := range t.Fill {
		if _, err := columnIndex(letter); err != nil {
			problems = append(problems, fmt.Sprintf("fill: %v", err))
		}
		if header, ok := t.Columns[letter]; ok {
			problems = append(problems, fmt.Sprintf("fill: the column %s is the one of %q, which the import writes", letter, header))
		}
	}
	seen := map[string]int{}
	for i, row := range t.Rows {
		where := fmt.Sprintf("rows[%d]", i)
		for _, header := range sortedKeys(row) {
			switch {
			case headers[header] == "":
				problems = append(problems, fmt.Sprintf("%s: %q is not a header of the columns", where, header))
				continue
			case t.Formulas[header] != "":
				problems = append(problems, fmt.Sprintf("%s: %q has the formula of its column", where, header))
				continue
			}
			if err := checkValue(row[header]); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %s: %v", where, header, err))
			}
		}
		values := make([]string, len(t.Keys))
		missing := false
		for j, k := range t.Keys {
			s, ok := row[k].(string)
			if !ok || s == "" || strings.HasPrefix(s, "=") {
				problems = append(problems, fmt.Sprintf("%s: no key %q, a text", where, k))
				missing = true
			}
			values[j] = s
		}
		if missing {
			continue
		}
		key := strings.Join(values, "\x1f")
		if j, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf("%s: the keys of rows[%d] too, %s", where, j, strings.Join(values, " ")))
		} else {
			seen[key] = i
		}
	}
	return problems
}

// Letters returns the letters of the columns of the tab, from left to right.
func (t *Tab) Letters() []string {
	letters := make([]string, 0, len(t.Columns))
	for letter := range t.Columns {
		letters = append(letters, letter)
	}
	sortLetters(letters)
	return letters
}

// LetterOf returns the letter of the column of a header, empty if the tab has no such column.
func (t *Tab) LetterOf(header string) string {
	for letter, h := range t.Columns {
		if h == header {
			return letter
		}
	}
	return ""
}

// Formula returns the formula of a cell at a row, with {row} replaced by its number.
func Formula(formula string, row int) string {
	return strings.ReplaceAll(formula, RowPlaceholder, fmt.Sprint(row))
}

// sortLetters sorts letters of columns from left to right: Z before AA.
func sortLetters(letters []string) {
	sort.Slice(letters, func(i, j int) bool {
		a, b := letters[i], letters[j]
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
}

// exportJSON and tabJSON are the layout of an export, whose rows are decoded by hand.
type exportJSON struct {
	Spreadsheet string            `json:"spreadsheet"`
	Tabs        []json.RawMessage `json:"tabs"`
}

type tabJSON struct {
	Name     string                       `json:"name"`
	Keys     []string                     `json:"keys"`
	Columns  map[string]string            `json:"columns"`
	Formulas map[string]string            `json:"formulas"`
	Fill     []string                     `json:"fill"`
	Rows     []map[string]json.RawMessage `json:"rows"`
}

// UnmarshalJSON decodes an export, its numbers as json.Number. An unknown attribute is an error,
// so that a typo doesn't go unnoticed.
func (e *Export) UnmarshalJSON(data []byte) error {
	var raw exportJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	*e = Export{Spreadsheet: raw.Spreadsheet}
	for i, data := range raw.Tabs {
		var t tabJSON
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&t); err != nil {
			return fmt.Errorf("tabs[%d]: %w", i, err)
		}
		tab := &Tab{Name: t.Name, Keys: t.Keys, Columns: t.Columns, Formulas: t.Formulas, Fill: t.Fill}
		for j, attributes := range t.Rows {
			row := make(map[string]any, len(attributes))
			for _, name := range sortedKeys(attributes) {
				d := json.NewDecoder(bytes.NewReader(attributes[name]))
				d.UseNumber()
				var v any
				if err := d.Decode(&v); err != nil {
					return fmt.Errorf("tabs[%d]: rows[%d]: %s: %w", i, j, name, err)
				}
				row[name] = v
			}
			tab.Rows = append(tab.Rows, row)
		}
		e.Tabs = append(e.Tabs, tab)
	}
	return nil
}

// MarshalJSON encodes an export: the columns from left to right, and the cells of each row in the
// order of the columns.
func (e *Export) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`{"spreadsheet":`)
	writeJSON(&b, e.Spreadsheet)
	b.WriteString(`,"tabs":[`)
	for i, t := range e.Tabs {
		if i > 0 {
			b.WriteByte(',')
		}
		t.marshal(&b)
	}
	b.WriteString(`]}`)
	return b.Bytes(), nil
}

func (t *Tab) marshal(b *bytes.Buffer) {
	b.WriteString(`{"name":`)
	writeJSON(b, t.Name)
	b.WriteString(`,"keys":`)
	writeJSON(b, nonNil(t.Keys))
	b.WriteString(`,"columns":{`)
	for i, letter := range t.Letters() {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSON(b, letter)
		b.WriteByte(':')
		writeJSON(b, t.Columns[letter])
	}
	b.WriteString(`},"formulas":{`)
	for i, header := range t.inColumnOrder(sortedKeys(t.Formulas)) {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSON(b, header)
		b.WriteByte(':')
		writeJSON(b, t.Formulas[header])
	}
	b.WriteString(`},"fill":`)
	writeJSON(b, nonNil(t.Fill))
	b.WriteString(`,"rows":[`)
	for i, row := range t.Rows {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('{')
		for j, header := range t.inColumnOrder(sortedKeys(row)) {
			if j > 0 {
				b.WriteByte(',')
			}
			writeJSON(b, header)
			b.WriteByte(':')
			writeJSON(b, row[header])
		}
		b.WriteByte('}')
	}
	b.WriteString(`]}`)
}

// inColumnOrder sorts the headers in the order of the columns, and returns them.
func (t *Tab) inColumnOrder(headers []string) []string {
	columns := map[string]int{}
	for letter, header := range t.Columns {
		if index, err := columnIndex(letter); err == nil {
			columns[header] = index
		}
	}
	sort.SliceStable(headers, func(i, j int) bool { return columns[headers[i]] < columns[headers[j]] })
	return headers
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// checkValue returns an error if v is not a value of a cell: a non-empty text, a number or a
// boolean.
func checkValue(v any) error {
	switch v := v.(type) {
	case string:
		if v == "" {
			return errors.New(`an empty text "": leave the header out for an empty cell`)
		}
	case json.Number, bool:
	default:
		return fmt.Errorf("%v is not a text, a number or a boolean", v)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// writeJSON writes v in JSON, without escaping <, > and &, which a comment may hold.
func writeJSON(b *bytes.Buffer, v any) {
	e := json.NewEncoder(b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)         // Strings, numbers, booleans and slices of strings: it never fails.
	b.Truncate(b.Len() - 1) // The newline of Encode.
}

var sheetID = regexp.MustCompile(`/spreadsheets/d/([A-Za-z0-9_-]+)`)

// SpreadsheetID returns the ID of a spreadsheet from its URL, empty if the URL is not one.
func SpreadsheetID(url string) string {
	if m := sheetID.FindStringSubmatch(url); m != nil {
		return m[1]
	}
	return ""
}
