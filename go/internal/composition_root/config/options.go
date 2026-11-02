// Package config holds the options with which a project chooses the adapters of the core in code (REQ-ARC-013,
// ADR-013). It is not the Config of the consumer: that one is infrastructure/config.
package config

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/services"
)

// =============================================================================
// OPTIONS
// =============================================================================

// Options are the adapters a project passed to the core. A nil Clock or IDGenerator means that the composition root
// uses its default.
type Options struct {
	Destinations services.Destinations
	Spool        outbound.LocalSpoolPort
	Clock        outbound.ClockPort
	IDGenerator  outbound.IDGeneratorPort
}

// Option sets one thing of the Options. An option whose argument is nil changes nothing.
type Option func(options *Options)

// ----- OPTIONS --------------------------------------------------------------

// WithLogSink adds a destination for the log events. Several sinks of the same kind are allowed (REQ-OUT-003).
func WithLogSink(sink outbound.LogSinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.Log = append(options.Destinations.Log, sink)
		}
	}
}

// WithEventSink adds a destination for the events that have no kind of their own.
func WithEventSink(sink outbound.EventSinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.Event = append(options.Destinations.Event, sink)
		}
	}
}

// WithMetricsSink adds a destination for the metric events.
func WithMetricsSink(sink outbound.MetricsSinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.Metrics = append(options.Destinations.Metrics, sink)
		}
	}
}

// WithTraceSink adds a destination for the trace events.
func WithTraceSink(sink outbound.TraceSinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.Trace = append(options.Destinations.Trace, sink)
		}
	}
}

// WithDeviceTelemetrySink adds a destination for the device telemetry events.
func WithDeviceTelemetrySink(sink outbound.DeviceTelemetrySinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.DeviceTelemetry = append(options.Destinations.DeviceTelemetry, sink)
		}
	}
}

// WithAITelemetrySink adds a destination for the AI telemetry events.
func WithAITelemetrySink(sink outbound.AITelemetrySinkPort) Option {
	return func(options *Options) {
		if sink != nil {
			options.Destinations.AITelemetry = append(options.Destinations.AITelemetry, sink)
		}
	}
}

// WithLocalSpool sets the spool of the durable delivery mode.
func WithLocalSpool(spool outbound.LocalSpoolPort) Option {
	return func(options *Options) {
		if spool != nil {
			options.Spool = spool
		}
	}
}

// WithClock sets the clock, instead of the system clock.
func WithClock(clock outbound.ClockPort) Option {
	return func(options *Options) {
		if clock != nil {
			options.Clock = clock
		}
	}
}

// WithIDGenerator sets the identifier generator, instead of the UUIDv7 generator.
func WithIDGenerator(id_generator outbound.IDGeneratorPort) Option {
	return func(options *Options) {
		if id_generator != nil {
			options.IDGenerator = id_generator
		}
	}
}
