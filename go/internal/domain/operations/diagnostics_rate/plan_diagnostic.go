// Package diagnostics_rate holds the pure rule that limits how many diagnostics are published (REQ-DGN-003): a fixed
// window with a number of diagnostics allowed in it. It reads no clock: the caller gives the time and keeps the state.
package diagnostics_rate

// =============================================================================
// PLAN DIAGNOSTIC
// =============================================================================

// NoWindow is the start of the window while there is none yet.
const NoWindow int64 = -1

// Plan is the answer of PlanDiagnostic: whether the diagnostic is allowed, the new state of the limiter (WindowStartMS,
// Count, Suppressed) and AttachSuppressed, the number of diagnostics that were suppressed before this one, which an
// allowed diagnostic carries (it is zero for a suppressed one).
type Plan struct {
	Allow            bool
	WindowStartMS    int64
	Count            int
	Suppressed       int
	AttachSuppressed int
}

// PlanDiagnostic decides whether the diagnostic at now_ms may be published. It is pure. The window is fixed: when there
// is none (window_start_ms is NoWindow) or now_ms - window_start_ms >= window_ms, a new window starts at now_ms with count
// 0. Then, when count is below limit the call is allowed, count grows by one, attach_suppressed is the number suppressed
// so far and that number goes back to 0; otherwise the call is suppressed (allow false, attach_suppressed 0) and the
// number suppressed grows by one. A clock that goes back stays in the window. The times are milliseconds of the same
// monotonic clock.
func PlanDiagnostic(now_ms int64, window_start_ms int64, count int, suppressed int, limit int, window_ms int64) Plan {
	if window_start_ms == NoWindow || now_ms-window_start_ms >= window_ms {
		window_start_ms = now_ms
		count = 0
	}
	if count < limit {
		return Plan{Allow: true, WindowStartMS: window_start_ms, Count: count + 1, Suppressed: 0, AttachSuppressed: suppressed}
	}
	return Plan{Allow: false, WindowStartMS: window_start_ms, Count: count, Suppressed: suppressed + 1}
}
