package common

import "strings"

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
