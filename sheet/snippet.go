package sheet

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

//go:embed snippet.js
var snippetJS string

// SnippetOptions are the options of Snippet.
type SnippetOptions struct {
	// Sheet is the URL of the spreadsheet: the script refuses to run in another one. Optional.
	Sheet string
	// Path is the file where the teacher saves what the script copies, for its message.
	Path string
}

var sheetID = regexp.MustCompile(`/spreadsheets/d/([A-Za-z0-9_-]+)`)

// Snippet writes the script that reads the tab of the file f in a browser, from the page of the
// spreadsheet. It reads the columns of the file only, with the query language of Google
// Visualization (gviz), and copies the rows to the clipboard, as a file of this package whose
// columns are the headers of the tab. A value that doesn't fit the type of most of its column,
// such as a text in a column of numbers, comes back empty: gviz types a column by its majority.
func Snippet(w io.Writer, f *File, opts SnippetOptions) error {
	if len(f.Columns) == 0 {
		return fmt.Errorf("no column to read in the tab %q", f.Tab)
	}
	letters := f.letters()
	columns := make([]string, 0, len(letters))
	for _, letter := range letters {
		header, err := json.Marshal(f.Columns[letter])
		if err != nil {
			return err
		}
		l, _ := json.Marshal(letter) // A string: it never fails.
		columns = append(columns, string(l)+": "+string(header))
	}
	id := ""
	if m := sheetID.FindStringSubmatch(opts.Sheet); m != nil {
		id = m[1]
	}
	values := []any{f.Tab, f.Keys, id, opts.Path}
	encoded := make([]string, len(values))
	for i, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		encoded[i] = string(b)
	}
	config := fmt.Sprintf("{\n  tab: %s,\n  keys: %s,\n  columns: { %s },\n  sheet: %s,\n  path: %s,\n}",
		encoded[0], encoded[1], strings.Join(columns, ", "), encoded[2], encoded[3])
	_, err := fmt.Fprintf(w, "// mc sheet snippet: reads the columns %s of the tab %q, and copies its rows to the\n// clipboard as JSON. Run it in the page of the spreadsheet.\n%s",
		strings.Join(letters, ", "), f.Tab, strings.Replace(snippetJS, "CONFIG", config, 1))
	return err
}
