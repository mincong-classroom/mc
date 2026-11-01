package sheet

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// pasteText returns the text to paste for a value, which Sheets reads as if it was typed: a
// formula stays a formula, and a text that Sheets would read as something else, such as the
// commit 2e88112 (a number) or -0.2, starts with an apostrophe, which keeps it a text.
func pasteText(v any) (string, error) {
	switch v := v.(type) {
	case bool:
		if v {
			return "TRUE", nil
		}
		return "FALSE", nil
	case json.Number:
		return v.String(), nil
	case string:
		if v == "" {
			return "", errors.New(`an empty text "" would clear the cell`)
		}
		if strings.ContainsAny(v, "\t\r\n") {
			return "", fmt.Errorf("%q: a tab or a line break would split the cell in a paste", v)
		}
		if strings.HasPrefix(v, "=") {
			return v, nil
		}
		if readAsNonText(v) {
			return "'" + v, nil
		}
		return v, nil
	}
	return "", fmt.Errorf("%v is not a text, a number or a boolean", v)
}

// dateOrTime matches a text that Sheets may read as a date or a time, such as 2026-10-30, 30/10
// or 14:28.
var dateOrTime = regexp.MustCompile(`^\d{1,4}[-/.]\d{1,2}([-/.]\d{1,4})?([ T]\d{1,2}:\d{2}(:\d{2})?)?$|^\d{1,2}:\d{2}(:\d{2})?( ?[AaPp][Mm])?$`)

// readAsNonText reports whether Sheets would read a typed text as something else than a text: a
// formula, a number, a percentage, an amount, a boolean, a date or a time. It may answer yes for a
// text that Sheets keeps, which costs an apostrophe only, never a value.
func readAsNonText(s string) bool {
	if strings.ContainsAny(s[:1], "+-@'") {
		return true // A formula, a signed number, or an apostrophe that Sheets would remove.
	}
	t := strings.TrimSpace(s)
	if strings.EqualFold(t, "true") || strings.EqualFold(t, "false") {
		return true
	}
	t = strings.TrimSuffix(strings.TrimPrefix(t, "$"), "%")
	if isNumber(strings.ReplaceAll(t, ",", "")) {
		return true
	}
	return dateOrTime.MatchString(t)
}

// isNumber reports whether s is a number, even one too large for a float64, such as 2e88112.
func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	var numErr *strconv.NumError
	return err == nil || errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange)
}

// text returns a value as a text, to show it in a message.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return "empty"
	case string:
		return strconv.Quote(v)
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	}
	return fmt.Sprint(v)
}

// SameValue reports whether the value of a cell read from the tab is the one written. A tab read
// as CSV gives texts, so a number or a boolean is compared with its text. A formula is never
// compared: the tab gives its result.
func SameValue(read, written any) bool {
	switch w := written.(type) {
	case json.Number:
		want, err := w.Float64()
		if err != nil {
			return false
		}
		got, ok := number(read)
		return ok && math.Abs(got-want) < 1e-9
	case bool:
		got := strings.TrimSpace(fmt.Sprint(read))
		return strings.EqualFold(got, strconv.FormatBool(w))
	case string:
		if strings.HasPrefix(w, "=") {
			return true
		}
		got, ok := read.(string)
		return ok && strings.TrimSpace(got) == strings.TrimSpace(w)
	}
	return false
}

// number returns the number of a value read from the tab, a json.Number or a text.
func number(v any) (float64, bool) {
	var s string
	switch v := v.(type) {
	case json.Number:
		s = v.String()
	case string:
		s = strings.TrimSpace(v)
	default:
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}
