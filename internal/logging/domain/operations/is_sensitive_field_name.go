package operations

import (
	"regexp"
	"strings"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
)

var field_name_camel_boundary = regexp.MustCompile(domain.FieldNameCamelBoundaryPattern)

// IsSensitiveFieldName reports whether the value of a field with this name must always be hidden. The name is read
// in snake case (accessKey, Access-Key and access_key are the same) and matches when it is equal to one of
// domain.SensitiveFieldNames or ends with "_" and one of them.
func IsSensitiveFieldName(field_name string) bool {
	normalized_name := field_name_camel_boundary.ReplaceAllString(field_name, "${1}_${2}")
	normalized_name = strings.ToLower(strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(normalized_name))

	for _, sensitive_name := range domain.SensitiveFieldNames {
		if normalized_name == sensitive_name || strings.HasSuffix(normalized_name, "_"+sensitive_name) {
			return true
		}
	}
	return false
}
