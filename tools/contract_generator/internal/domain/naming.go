package domain

import "strings"

// words splits a name in snake_case, or an event type with dots, in its lowercase words.
func words(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(char rune) bool { return char == '_' || char == '.' })
}

// PascalCase joins the words of a name with each one capitalized (log.entry.recorded gives LogEntryRecorded). It is
// the form of Go identifiers and of Python classes (contracts/conventions/naming.md).
func PascalCase(name string) string {
	var joined strings.Builder
	for _, word := range words(name) {
		joined.WriteString(strings.ToUpper(word[:1]) + word[1:])
	}
	return joined.String()
}

// UpperSnake joins the words of a name in capitals with underscores (log.entry.recorded gives LOG_ENTRY_RECORDED).
// It is the form of Python constants and enum members.
func UpperSnake(name string) string {
	return strings.ToUpper(strings.Join(words(name), "_"))
}
