// Package masking hides sensitive data in the data of an event before the event reaches any sink (ADR-006). It holds
// the detectors of text, the rule of the sensitive field names, the classification of fields and the masker that joins
// them. It uses the standard library and the domain only, and it does no I/O.
package masking

import "regexp"

// =============================================================================
// TEXT DETECTOR
// =============================================================================

// TextDetector finds one kind of sensitive data inside a text and hides it. Every detector of the masker is a
// TextDetector built by its own file (detect_*.go) from the patterns of domain/rules.go.
type TextDetector struct {
	name        string
	pattern     *regexp.Regexp
	replacement string
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewTextDetector creates a TextDetector; replacement may use ${1}-style references to the groups of the pattern.
func NewTextDetector(name string, pattern *regexp.Regexp, replacement string) TextDetector {
	return TextDetector{name: name, pattern: pattern, replacement: replacement}
}

// ----- METHODS --------------------------------------------------------------

// Name returns the name of the detector.
func (text_detector TextDetector) Name() string {
	return text_detector.name
}

// Apply returns the text with everything the detector finds hidden; the text itself is not modified.
func (text_detector TextDetector) Apply(text string) string {
	return text_detector.pattern.ReplaceAllString(text, text_detector.replacement)
}
