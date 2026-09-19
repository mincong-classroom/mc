package common

import (
	"fmt"
	"strings"
	"unicode"
)

// Name is a person's name in the format "LAST First": the last name in upper case, then the
// first name in Pascal case, each of one or more words, e.g. "DE LA FONTAINE Jean-Pierre".
type Name struct {
	Last  string
	First string
}

// ParseName parses a name in the format "LAST First". The last name is the leading words in upper
// case, and the first name the other words, each in Pascal case.
func ParseName(s string) (Name, error) {
	if strings.Contains(s, ",") {
		return Name{}, fmt.Errorf("%q is not \"LAST First\": remove the comma", s)
	}
	words := strings.Fields(s)
	i := 0
	for i < len(words) && isUpperCaseWord(words[i]) {
		i++
	}
	switch {
	case i == 0:
		return Name{}, fmt.Errorf("%q is not \"LAST First\": it must start with the last name in upper case", s)
	case i == len(words):
		return Name{}, fmt.Errorf("%q is not \"LAST First\": the first name is missing", s)
	}
	for _, word := range words[i:] {
		if !isPascalCaseWord(word) {
			return Name{}, fmt.Errorf("%q is not \"LAST First\": %q is not in Pascal case, such as \"John\"", s, word)
		}
	}
	return Name{Last: strings.Join(words[:i], " "), First: strings.Join(words[i:], " ")}, nil
}

func (n Name) String() string {
	return n.Last + " " + n.First
}

// isUpperCaseWord reports whether the word has letters, all in upper case, such as "SMITH",
// "D'ALMEIDA" or "DUPONT-MOREAU".
func isUpperCaseWord(word string) bool {
	hasLetter := false
	for _, r := range word {
		switch {
		case unicode.IsLetter(r):
			if !unicode.IsUpper(r) {
				return false
			}
			hasLetter = true
		case r != '-' && r != '\'':
			return false
		}
	}
	return hasLetter
}

// isPascalCaseWord reports whether each part of the word, separated by "-" or "'", is a letter in
// upper case followed by letters in lower case, such as "John", "Jean-Pierre" or "Soudémè".
func isPascalCaseWord(word string) bool {
	parts := strings.FieldsFunc(word, func(r rune) bool { return r == '-' || r == '\'' })
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		for i, r := range []rune(part) {
			if i == 0 && !unicode.IsUpper(r) || i > 0 && !unicode.IsLower(r) {
				return false
			}
		}
	}
	return true
}

// SameName reports whether two names designate the same person, ignoring the case, the extra
// spaces, and the comma of the former format "LAST, First".
func SameName(a, b string) bool {
	return normalizeName(a) == normalizeName(b)
}

func normalizeName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(name, ",", " ")), " "))
}
