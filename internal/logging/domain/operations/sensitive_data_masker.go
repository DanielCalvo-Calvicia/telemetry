package operations

import (
	"regexp"

	domain "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain"
	entities "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/entities"
	value_objects "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/value_objects"
)

// =============================================================================
// SENSITIVE DATA MASKER OPERATION
// =============================================================================

// ----------------------------------------------------------------------------
// ERRORS
// ----------------------------------------------------------------------------
var (
	ErrMaskPatternInvalid = domain.ErrMaskPatternInvalid
)

// SensitiveDataMasker hides sensitive data in a log entry before the entry leaves the logger, so that every
// project that imports telemetry gets already-masked output. It finds sensitive data in three shapes: a single
// value (by the name of its field or by what the value is), a whole text and a segment inside a longer text.
// Whatever is found is replaced by domain.MaskedText. The masker never modifies what it receives: it returns
// masked copies. It is immutable and safe for concurrent use.
type SensitiveDataMasker struct {
	text_detectors []TextDetector
}

// ----------------------------------------------------------------------------
// CONSTRUCTORS
// ----------------------------------------------------------------------------

// NewSensitiveDataMasker creates a masker with the built-in detectors plus one detector per extra pattern
// (regular expressions from the configuration, LOGGER_MASK_EXTRA_PATTERNS); everything an extra pattern
// matches is hidden. A pattern that is not a valid regular expression returns ErrMaskPatternInvalid with the
// compile error, so the start of the application fails instead of logging without the protection.
func NewSensitiveDataMasker(extra_patterns []string) (SensitiveDataMasker, error) {
	text_detectors := []TextDetector{
		NewPrivateKeyBlockDetector(),
		NewConnectionStringPasswordDetector(),
		NewBearerCredentialDetector(),
		NewJWTDetector(),
		NewAWSAccessKeyDetector(),
		NewCredentialPairDetector(),
	}

	for _, extra_pattern := range extra_patterns {
		if extra_pattern == "" {
			continue
		}
		compiled_pattern, err := regexp.Compile(extra_pattern)
		if err != nil {
			return SensitiveDataMasker{}, domain.JoinErrors(err, ErrMaskPatternInvalid)
		}
		text_detectors = append(text_detectors, NewTextDetector(extra_pattern, compiled_pattern, domain.MaskedText))
	}

	return SensitiveDataMasker{text_detectors: text_detectors}, nil
}

// ----------------------------------------------------------------------------
// METHODS
// ----------------------------------------------------------------------------

// MaskText returns the text with every sensitive segment hidden, whether the segment is the whole text or a
// part of it. Masking is idempotent: masking an already masked text changes nothing.
func (sensitive_data_masker SensitiveDataMasker) MaskText(text string) string {
	for _, text_detector := range sensitive_data_masker.text_detectors {
		text = text_detector.Apply(text)
	}
	return text
}

// MaskFields returns masked copies of the fields (the maps and lists inside them are copied too); the original
// fields are not modified. Nil fields give nil.
func (sensitive_data_masker SensitiveDataMasker) MaskFields(fields value_objects.Fields) value_objects.Fields {
	if fields == nil {
		return nil
	}
	return value_objects.Fields(sensitive_data_masker.maskMap(fields))
}

// MaskLogEntry returns a copy of the entry with its message and fields masked; the original entry and its fields
// are not modified.
func (sensitive_data_masker SensitiveDataMasker) MaskLogEntry(log_entry entities.LogEntry) entities.LogEntry {
	log_entry.Message = value_objects.NewMessage(sensitive_data_masker.MaskText(log_entry.Message.String()))
	log_entry.Fields = sensitive_data_masker.MaskFields(log_entry.Fields)
	return log_entry
}
