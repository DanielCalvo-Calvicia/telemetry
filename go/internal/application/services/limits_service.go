package services

import (
	"context"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/operations/limits"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
)

// =============================================================================
// LIMITS SERVICE
// =============================================================================

// LimitsService applies the size limits to an envelope (REQ-CTR-010). It runs AFTER masking, so a secret is never cut
// in half and left half visible (ADR-006), and BEFORE the sinks; T-03.9 wires the chain. The counters are returned with
// the result and are exposed as diagnostics by T-05.5; a rejected event is counted by the caller from the report.
type LimitsService struct {
	limits value_objects.Limits
}

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewLimitsService returns a LimitsService that applies the limits.
func NewLimitsService(limits value_objects.Limits) *LimitsService {
	return &LimitsService{limits: limits}
}

// ----- METHODS --------------------------------------------------------------

// Apply returns a limited copy of the envelope and the report of what was cut, by the policy of limits.ApplyLimits; the
// envelope it receives is never modified. An event that is still too large after the cuts returns the classified error
// limit_exceeded, an empty envelope and a report with EventsRejected = 1. It never panics: a panic becomes an empty
// envelope, a zero report and a classified error (limits_failed) that wraps application.ErrEnvelopeLimitsFailed, and the caller drops the event
// (ADR-005).
func (service *LimitsService) Apply(_ context.Context, envelope value_objects.Envelope) (limited_envelope value_objects.Envelope, report limits.Report, apply_error error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			limited_envelope = value_objects.Envelope{}
			report = limits.Report{}
			apply_error = domain.NewClassifiedError(value_objects.ErrorCodeLimitsFailed, application.ErrEnvelopeLimitsFailed, nil)
		}
	}()

	return limits.ApplyLimits(envelope, service.limits)
}
