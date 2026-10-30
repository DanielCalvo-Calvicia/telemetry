// Package limits applies the size limits to an event (REQ-CTR-010, REQ-SEC-004): it cuts the attributes by a documented
// policy, rejects an event that is still too large and says whether an event fits in a batch. It counts what it cut and
// never keeps the cut text. It uses the standard library and the domain only and does no I/O (REQ-ARC-005).
package limits

import (
	"errors"
	"strconv"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/canonical_json"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// ===== ApplyLimits ==========================================================

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrEventTooLarge is domain.ErrEventTooLarge.
	ErrEventTooLarge = domain.ErrEventTooLarge
)

// ----- OPERATIONS -----------------------------------------------------------

// ApplyLimits returns a limited copy of the envelope and the report of what was cut; the envelope it receives is never
// modified. Only envelope.Attributes is cut; data and every other field are not touched. The policy (REQ-CTR-010):
//
//  1. The keys are taken in the sorted order of the canonical form (UTF-16 code units of the original key), because a
//     Go map has no insertion order and this order is the same in Go and in Python. The keys beyond the first
//     MaxAttributeKeys are dropped and counted in AttributeKeysDropped.
//  2. A kept key longer than MaxAttributeKeyChars characters is cut to that length and counted in
//     AttributeKeysTruncated. When the cut key is equal to a key that is already kept, this key is dropped and counted
//     only in AttributeKeysDropped.
//  3. A text value longer than MaxAttributeValueChars characters is cut to that length and counted in
//     AttributeValuesTruncated; each text of a list is limited and counted on its own. Numbers, booleans and null are
//     never cut.
//  4. A character is a Unicode code point: never a byte and never a UTF-16 unit, so 257 emoji are cut to 256 emoji.
//  5. When no key remains the attributes are omitted (nil).
//  6. After the cuts the size of the event is the length in bytes of its canonical JSON. When it is greater than
//     MaxEventBytes, the result is the empty Envelope, the report keeps the other counters and has EventsRejected = 1,
//     and the error is a classified error with the code limit_exceeded that wraps ErrEventTooLarge; its detail holds the
//     size and the limit, never the content. An event of exactly MaxEventBytes is accepted.
//
// An error of the canonical JSON (an unsupported value) is returned as it is, with the empty Envelope.
func ApplyLimits(envelope value_objects.Envelope, limits value_objects.Limits) (value_objects.Envelope, Report, error) {
	limited_envelope := envelope
	limited_attributes, report := cut_attributes(envelope.Attributes, limits)
	limited_envelope.Attributes = limited_attributes

	canonical_bytes, canonical_error := canonical_json.CanonicalJSON(limited_envelope)
	if canonical_error != nil {
		return value_objects.Envelope{}, report, canonical_error
	}
	if len(canonical_bytes) > limits.MaxEventBytes() {
		report.EventsRejected = 1
		detail := errors.New("the event has " + strconv.Itoa(len(canonical_bytes)) + " bytes and the limit is " + strconv.Itoa(limits.MaxEventBytes()))
		return value_objects.Envelope{}, report, domain.NewClassifiedError(value_objects.ErrorCodeLimitExceeded, ErrEventTooLarge, detail)
	}
	return limited_envelope, report, nil
}
