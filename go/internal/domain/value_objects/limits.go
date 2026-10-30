package value_objects

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
)

// ===== Limits ===============================================================

// Limits are the size limits of an event: the number of attributes, the length of a key and of a text value in
// characters, the size of one event and the size of one HTTP batch in bytes of canonical JSON (REQ-CTR-010,
// REQ-SEC-004). The value is immutable: it has no setter, and NewLimits checks every number once.
//
// The zero value of Limits is not valid (all its limits are zero): get the defaults with DefaultLimits and a
// configured value with NewLimits. T-03.6 maps the configuration of the consumer to NewLimits.
type Limits struct {
	max_attribute_keys        int
	max_attribute_key_chars   int
	max_attribute_value_chars int
	max_event_bytes           int
	max_batch_bytes           int
}

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrLimitInvalid is domain.ErrLimitInvalid.
	ErrLimitInvalid = domain.ErrLimitInvalid
	// ErrBatchLimitBelowEventLimit is domain.ErrBatchLimitBelowEventLimit.
	ErrBatchLimitBelowEventLimit = domain.ErrBatchLimitBelowEventLimit
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// DefaultLimits returns the default limits: 32 attributes, a key of 64 characters, a value of 256 characters, an event
// of 64 KiB and a batch of 1 MiB.
func DefaultLimits() Limits {
	return Limits{
		max_attribute_keys:        domain.DefaultMaxAttributeKeys,
		max_attribute_key_chars:   domain.DefaultMaxAttributeKeyChars,
		max_attribute_value_chars: domain.DefaultMaxAttributeValueChars,
		max_event_bytes:           domain.DefaultMaxEventBytes,
		max_batch_bytes:           domain.DefaultMaxBatchBytes,
	}
}

// NewLimits returns the Limits of the numbers. Every number must be greater than zero, otherwise it returns a
// classified error with the code config_field_invalid that wraps ErrLimitInvalid; the limit of the batch must be at
// least the limit of the event, otherwise the error wraps ErrBatchLimitBelowEventLimit.
func NewLimits(max_attribute_keys int, max_attribute_key_chars int, max_attribute_value_chars int, max_event_bytes int, max_batch_bytes int) (Limits, error) {
	for _, limit := range []int{max_attribute_keys, max_attribute_key_chars, max_attribute_value_chars, max_event_bytes, max_batch_bytes} {
		if limit <= 0 {
			return Limits{}, domain.NewClassifiedError(ErrorCodeConfigFieldInvalid, ErrLimitInvalid, nil)
		}
	}
	if max_batch_bytes < max_event_bytes {
		return Limits{}, domain.NewClassifiedError(ErrorCodeConfigFieldInvalid, ErrBatchLimitBelowEventLimit, nil)
	}
	return Limits{
		max_attribute_keys:        max_attribute_keys,
		max_attribute_key_chars:   max_attribute_key_chars,
		max_attribute_value_chars: max_attribute_value_chars,
		max_event_bytes:           max_event_bytes,
		max_batch_bytes:           max_batch_bytes,
	}, nil
}

// ----- METHODS --------------------------------------------------------------

// MaxAttributeKeys returns the number of attributes that an event may keep.
func (limits Limits) MaxAttributeKeys() int {
	return limits.max_attribute_keys
}

// MaxAttributeKeyChars returns the length of an attribute key, in characters.
func (limits Limits) MaxAttributeKeyChars() int {
	return limits.max_attribute_key_chars
}

// MaxAttributeValueChars returns the length of a text value of an attribute, in characters.
func (limits Limits) MaxAttributeValueChars() int {
	return limits.max_attribute_value_chars
}

// MaxEventBytes returns the size of one event, in bytes of canonical JSON.
func (limits Limits) MaxEventBytes() int {
	return limits.max_event_bytes
}

// MaxBatchBytes returns the size of one HTTP batch, in bytes.
func (limits Limits) MaxBatchBytes() int {
	return limits.max_batch_bytes
}
