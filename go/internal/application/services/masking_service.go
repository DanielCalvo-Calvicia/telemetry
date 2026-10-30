package services

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/masking"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// MASKING SERVICE
// =============================================================================

// MaskingService hides the sensitive data of an envelope once, before it reaches any sink (ADR-006). No path may send
// an envelope to a sink without going through Mask (ADR-006); T-03.9 wires the chain.
type MaskingService struct {
	masker masking.SensitiveDataMasker
}

// ----- CONSTANTS ------------------------------------------------------------

// The names of the fields of the context, read by the masker like the name of any other field.
const (
	trace_id_field       = "trace_id"
	span_id_field        = "span_id"
	execution_id_field   = "execution_id"
	correlation_id_field = "correlation_id"
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewMaskingService returns a MaskingService that masks with the masker.
func NewMaskingService(masker masking.SensitiveDataMasker) *MaskingService {
	return &MaskingService{masker: masker}
}

// ----- METHODS --------------------------------------------------------------

// Mask returns a masked copy of the envelope; the envelope it receives is never modified. Only data, attributes and
// context are masked (data and attributes by the masker, field by field, and the four ids of the context by the rules
// of their names); event_type, the ids of the event, the times, the severity, the schema version, the resource and the
// actor are never changed. It never panics: a panic becomes an empty envelope and a classified error (masking_failed)
// that wraps
// application.ErrEnvelopeMaskingFailed, and the caller drops the event (ADR-005). It never returns the unmasked
// envelope.
func (service *MaskingService) Mask(_ context.Context, envelope value_objects.Envelope) (masked_envelope value_objects.Envelope, mask_error error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			masked_envelope = value_objects.Envelope{}
			mask_error = domain.NewClassifiedError(value_objects.ErrorCodeMaskingFailed, application.ErrEnvelopeMaskingFailed, nil)
		}
	}()

	masked_envelope = envelope
	masked_envelope.Data = service.masker.MaskMap(envelope.Data)
	masked_envelope.Attributes = mask_attributes(service.masker, envelope.Attributes)
	masked_envelope.Context = mask_context(service.masker, envelope.Context)
	if envelope.Actor != nil {
		copied_actor := *envelope.Actor
		masked_envelope.Actor = &copied_actor
	}
	return masked_envelope, nil
}

// ----- HELPERS --------------------------------------------------------------

// mask_attributes returns the masked copy of the attributes; nil stays nil.
func mask_attributes(masker masking.SensitiveDataMasker, attributes value_objects.Attributes) value_objects.Attributes {
	if attributes == nil {
		return nil
	}
	return value_objects.Attributes(masker.MaskMap(attributes))
}

// mask_context returns the copy of the context with its four ids masked; nil stays nil.
func mask_context(masker masking.SensitiveDataMasker, context_value *value_objects.Context) *value_objects.Context {
	if context_value == nil {
		return nil
	}
	return &value_objects.Context{
		TraceId:       mask_context_text(masker, trace_id_field, context_value.TraceId),
		SpanId:        mask_context_text(masker, span_id_field, context_value.SpanId),
		ExecutionId:   mask_context_text(masker, execution_id_field, context_value.ExecutionId),
		CorrelationId: mask_context_text(masker, correlation_id_field, context_value.CorrelationId),
	}
}

// mask_context_text masks one id of the context as the value of the field of that name.
func mask_context_text(masker masking.SensitiveDataMasker, field_name string, text string) string {
	masked_text, is_text := masker.MaskValue(field_name, text).(string)
	if !is_text {
		return domain.MaskedText
	}
	return masked_text
}
