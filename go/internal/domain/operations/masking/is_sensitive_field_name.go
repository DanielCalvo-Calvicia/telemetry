package masking

import (
	"regexp"
	"strings"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

var field_name_camel_boundary = regexp.MustCompile(domain.FieldNameCamelBoundaryPattern)

var field_name_separators = strings.NewReplacer("-", "_", " ", "_", ".", "_")

// default_field_names is the default list, normalized once.
var default_field_names = normalize_field_names(domain.SensitiveFieldNames)

// IsSensitiveFieldName reports whether the value of a field with this name must always be hidden. The name is read
// in snake case (accessKey, Access-Key and access_key are the same) and matches when it is equal to one of
// domain.SensitiveFieldNames (the default list) or ends with "_" and one of them. A masker made with options may use
// another list; see MaskerOptions.
func IsSensitiveFieldName(field_name string) bool {
	return matches_any_name(normalize_field_name(field_name), default_field_names)
}

// normalize_field_name reads a field name in snake case: a camel case boundary gets an underscore, the letters are
// lower case and "-", " " and "." become "_".
func normalize_field_name(field_name string) string {
	normalized_name := field_name_camel_boundary.ReplaceAllString(field_name, "${1}_${2}")
	return strings.ToLower(field_name_separators.Replace(normalized_name))
}

// normalize_field_names returns the normalized copy of a list of names, without the empty ones.
func normalize_field_names(field_names []string) []string {
	normalized_names := make([]string, 0, len(field_names))
	for _, field_name := range field_names {
		if normalized_name := normalize_field_name(field_name); normalized_name != "" {
			normalized_names = append(normalized_names, normalized_name)
		}
	}
	return normalized_names
}

// matches_any_name reports whether a normalized name is equal to one of the normalized names or ends with "_" and one
// of them.
func matches_any_name(normalized_name string, normalized_names []string) bool {
	for _, sensitive_name := range normalized_names {
		if normalized_name == sensitive_name || strings.HasSuffix(normalized_name, "_"+sensitive_name) {
			return true
		}
	}
	return false
}
