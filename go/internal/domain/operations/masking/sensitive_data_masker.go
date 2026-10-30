package masking

import (
	"errors"
	"regexp"
	"regexp/syntax"
	"strconv"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// SENSITIVE DATA MASKER
// =============================================================================

// SensitiveDataMasker hides sensitive data in the data of an event before the event leaves the core, so that every
// sink gets already-masked output (ADR-006). It finds sensitive data in three shapes: a single value (by the name of
// its field or by what the value is), a whole text and a segment inside a longer text. Whatever is found is replaced by
// domain.MaskedText. The masker never modifies what it receives: it returns masked copies. It is immutable and safe
// for concurrent use.
type SensitiveDataMasker struct {
	text_detectors []TextDetector
	field_names    []string
	class_rules    field_class_rules
}

// MaskerOptions are the settings of a masker. The zero value is the default masker: the six detectors of text and the
// default list of field names. T-03.6 maps the configuration of the consumer to this type; the core never reads a file
// or the environment.
type MaskerOptions struct {
	// ExtraPatterns are regular expressions whose whole match is hidden, on top of the six detectors that are always on.
	// An empty pattern is skipped.
	ExtraPatterns []string
	// FieldNames is the list of sensitive field names. nil means the default list (domain.SensitiveFieldNames); a list
	// that is not nil, even an empty one, REPLACES the default list. The six detectors of text stay on in both cases.
	FieldNames []string
	// ExtraFieldNames are added to the active list of field names, whether it is the default one or FieldNames.
	ExtraFieldNames []string
	// FieldClasses gives the class of a field name (the key is read in snake case, like every field name).
	FieldClasses map[string]string
	// ClassRules gives the rule of each class: domain.MaskRuleDrop, domain.MaskRuleRedact or domain.MaskRuleAllow. A
	// field whose class has no rule is redacted.
	ClassRules map[string]string
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrMaskPatternInvalid is domain.ErrMaskPatternInvalid.
	ErrMaskPatternInvalid = domain.ErrMaskPatternInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewSensitiveDataMasker creates a masker with the six built-in detectors, in their order, plus one detector per
// extra pattern; everything an extra pattern matches is hidden. An extra pattern that does not compile is a
// config_field_invalid error (domain.ClassifiedError) with ErrMaskPatternInvalid and the compile error, and a class
// rule that is not drop, redact or allow is the same code with ErrMaskRuleInvalid, so the start of the application
// fails instead of logging without the protection.
func NewSensitiveDataMasker(options MaskerOptions) (SensitiveDataMasker, error) {
	text_detectors := []TextDetector{
		NewPrivateKeyBlockDetector(),
		NewConnectionStringPasswordDetector(),
		NewBearerCredentialDetector(),
		NewJWTDetector(),
		NewAWSAccessKeyDetector(),
		NewCredentialPairDetector(),
	}

	for pattern_position, extra_pattern := range options.ExtraPatterns {
		if extra_pattern == "" {
			continue
		}
		compiled_pattern, compile_error := regexp.Compile(extra_pattern)
		if compile_error != nil {
			return SensitiveDataMasker{}, domain.NewClassifiedError(value_objects.ErrorCodeConfigFieldInvalid, ErrMaskPatternInvalid, pattern_detail(pattern_position, compile_error))
		}
		text_detectors = append(text_detectors, NewTextDetector(extra_pattern, compiled_pattern, domain.MaskedText))
	}

	class_rules, rules_error := new_field_class_rules(options.FieldClasses, options.ClassRules)
	if rules_error != nil {
		return SensitiveDataMasker{}, rules_error
	}

	active_names := default_field_names
	if options.FieldNames != nil {
		active_names = normalize_field_names(options.FieldNames)
	}
	field_names := append(append([]string{}, active_names...), normalize_field_names(options.ExtraFieldNames)...)

	return SensitiveDataMasker{text_detectors: text_detectors, field_names: field_names, class_rules: class_rules}, nil
}

// ----- METHODS --------------------------------------------------------------

// MaskText returns the text with every sensitive segment hidden, whether the segment is the whole text or a
// part of it. Masking is idempotent: masking an already masked text changes nothing.
func (sensitive_data_masker SensitiveDataMasker) MaskText(text string) string {
	for _, text_detector := range sensitive_data_masker.text_detectors {
		text = text_detector.Apply(text)
	}
	return text
}

// MaskMap returns a masked copy of the map (the maps and lists inside it are copied too); the original is not
// modified. The keys whose class has the rule drop are left out, at any depth. A nil map gives nil.
func (sensitive_data_masker SensitiveDataMasker) MaskMap(source_map map[string]any) map[string]any {
	if source_map == nil {
		return nil
	}
	return sensitive_data_masker.mask_map(source_map)
}

// ----- HELPERS --------------------------------------------------------------

// is_sensitive reports whether the value of the field is hidden by the active list of names.
func (sensitive_data_masker SensitiveDataMasker) is_sensitive(field_name string) bool {
	return matches_any_name(normalize_field_name(field_name), sensitive_data_masker.field_names)
}

// pattern_detail returns the detail of a pattern that does not compile: its position in the list and the reason of the
// parser, without the pattern.
// The error of the regexp package quotes the pattern, and a pattern may hold a secret, so it is never kept.
func pattern_detail(pattern_position int, compile_error error) error {
	var syntax_error *syntax.Error
	if errors.As(compile_error, &syntax_error) {
		return errors.New("extra pattern " + strconv.Itoa(pattern_position) + " is not valid: " + string(syntax_error.Code))
	}
	return errors.New("extra pattern " + strconv.Itoa(pattern_position) + " is not valid")
}
