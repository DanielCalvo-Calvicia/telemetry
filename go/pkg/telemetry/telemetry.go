// Package telemetry is the public entry point of the telemetry core. A project builds the core once with New, giving
// it a Config (start from a profile: Microservice, Cronjob, Worker, Robotics or OfflineDevice) and the adapters it
// chose, passed in code with the options WithLogSink, WithEventSink and the others; it then calls Emit for each event
// and Shutdown when it ends. Every event is built, validated, masked and limited before it reaches a sink, and a sink
// that fails never fails the caller. The package reads no environment variable and no file: the project loads its own
// configuration and passes it in. Nothing is global, so two cores in one process share nothing.
//
// The types are aliases of the types of the core (ADR-003); the functions are thin.
package telemetry

import (
	"context"
	"log/slog"
	"time"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/call_context"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/inbound"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/ports/outbound"
	config_options "github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/composition_root/containers"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"
	infra_config "github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/config"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/infrastructure/inbound/sdk"
)

// ----- TYPES: THE CORE AND ITS CONFIGURATION --------------------------------

// Telemetry is the core of one project, built by New. Its methods are Emit, Logger, Flush and Shutdown.
type Telemetry = containers.Core

// Config is the configuration of the core, in plain values. The project chooses a profile and sets what it wants.
type Config = infra_config.Config

// ResourceConfig is the identity of the emitting process.
type ResourceConfig = infra_config.ResourceConfig

// DeliveryConfig is how events leave the process.
type DeliveryConfig = infra_config.DeliveryConfig

// LimitsConfig is the size limits of an event and of a batch.
type LimitsConfig = infra_config.LimitsConfig

// Limits are the size limits of an event and of a batch as the adapters take them (the sink adapters that cap a batch,
// such as the HTTP destination, receive them). Get them with DefaultLimits or NewLimits; the zero value is not valid.
type Limits = value_objects.Limits

// DefaultLimits returns the default limits: 32 attributes, a key of 64 characters, a value of 256 characters, an event
// of 64 KiB and a batch of 1 MiB.
func DefaultLimits() Limits { return value_objects.DefaultLimits() }

// NewLimits returns the Limits of the numbers: the attributes of an event, the characters of a key, the characters of a
// text value, the bytes of an event and the bytes of a batch. Every number must be greater than zero and the batch limit
// not below the event limit, otherwise it returns an error.
func NewLimits(max_attribute_keys int, max_attribute_key_chars int, max_attribute_value_chars int, max_event_bytes int, max_batch_bytes int) (Limits, error) {
	return value_objects.NewLimits(max_attribute_keys, max_attribute_key_chars, max_attribute_value_chars, max_event_bytes, max_batch_bytes)
}

// SpoolConfig is the local spool of the durable delivery mode.
type SpoolConfig = infra_config.SpoolConfig

// MaskingConfig is the options of the masking.
type MaskingConfig = infra_config.MaskingConfig

// ValidationError is what New returns for an invalid configuration: every problem, with its field and its code.
type ValidationError = infra_config.ValidationError

// Option chooses an adapter of the core in code.
type Option = config_options.Option

// ----- TYPES: INPUTS AND RESULTS --------------------------------------------

// NewEventInput is what a project gives the core to build one event.
type NewEventInput = dtos.NewEventInput

// EmitResult is what Emit answers: how many events were accepted, dropped or failed, and why.
type EmitResult = dtos.EmitResult

// ShutdownResult is what Shutdown answers: how many events were sent, spooled or dropped.
type ShutdownResult = dtos.ShutdownResult

// SendResult is what a sink answers to a call: how many events were accepted, dropped or failed.
type SendResult = dtos.SendResult

// Envelope is the event as it reaches a sink: complete, valid, masked and limited.
type Envelope = value_objects.Envelope

// Severity is the severity of an event.
type Severity = value_objects.Severity

// The severities of an event.
const (
	// SeverityTrace is the most detailed severity.
	SeverityTrace Severity = value_objects.SeverityTrace
	// SeverityDebug is the severity of diagnostic detail.
	SeverityDebug Severity = value_objects.SeverityDebug
	// SeverityInfo is the severity of normal events.
	SeverityInfo Severity = value_objects.SeverityInfo
	// SeverityWarn is the severity of something that may become a problem.
	SeverityWarn Severity = value_objects.SeverityWarn
	// SeverityError is the severity of a failure.
	SeverityError Severity = value_objects.SeverityError
	// SeverityCritical is the severity of a failure that needs attention at once.
	SeverityCritical Severity = value_objects.SeverityCritical
)

// EventAttributes are the free key-value pairs of an event.
type EventAttributes = value_objects.EventAttributes

// ActorIdentity says who produced an event.
type ActorIdentity = value_objects.ActorIdentity

// ActorOption sets an optional field of an ActorIdentity.
type ActorOption = value_objects.ActorOption

// UTCInstant is a moment in UTC, with microsecond precision.
type UTCInstant = value_objects.UTCInstant

// EventID is the id of an event: a UUID of version 7.
type EventID = value_objects.EventID

// Identifier is a text without control characters that is not blank, such as the instance id of a resource.
type Identifier = base_value_objects.Identifier

// CorrelationContext links an event to a trace and to the work that produced it.
type CorrelationContext = value_objects.CorrelationContext

// CallContext is the identity and the position of a call, carried in a context.Context.
type CallContext = call_context.CallContext

// CallContextOption sets a field of a CallContext.
type CallContextOption = call_context.Option

// ----- TYPES: THE LOGGER ----------------------------------------------------

// Logger is the logger of the core: Trace, Debug, Info, Warn, Error and Critical, each with a context and a LogEntry and
// no return value, because a log call never fails the caller. Get it from Telemetry.Logger.
type Logger = inbound.LoggerPort

// LogEntry is one log line: the category, the optional function, the message and the extra fields, which become the
// attributes of the event. No request body is recorded unless the caller passes it in the fields.
type LogEntry = dtos.LogEntryInput

// ----- TYPES: PORTS ---------------------------------------------------------

// ClockPort is the port of the clock; a project may pass its own with WithClock.
type ClockPort = outbound.ClockPort

// IDGeneratorPort is the port of the identifier generator; a project may pass its own with WithIDGenerator.
type IDGeneratorPort = outbound.IDGeneratorPort

// LogSinkPort is the port of a sink of log events.
type LogSinkPort = outbound.LogSinkPort

// EventSinkPort is the port of a sink of the events that have no kind of their own.
type EventSinkPort = outbound.EventSinkPort

// MetricsSinkPort is the port of a sink of metric events.
type MetricsSinkPort = outbound.MetricsSinkPort

// TraceSinkPort is the port of a sink of trace events.
type TraceSinkPort = outbound.TraceSinkPort

// DeviceTelemetrySinkPort is the port of a sink of device telemetry events.
type DeviceTelemetrySinkPort = outbound.DeviceTelemetrySinkPort

// AITelemetrySinkPort is the port of a sink of AI telemetry events.
type AITelemetrySinkPort = outbound.AITelemetrySinkPort

// LocalSpoolPort is the port of the local spool of the durable delivery mode.
type LocalSpoolPort = outbound.LocalSpoolPort

// ----- THE CORE -------------------------------------------------------------

// New validates the configuration and builds the core with the adapters of the options. An invalid configuration
// returns a *ValidationError with every problem; a durable delivery mode with no WithLocalSpool is one of them. The
// start uses context.Background(). Nothing is read from the environment.
func New(config Config, options ...Option) (*Telemetry, error) {
	return containers.NewCore(context.Background(), config, options...)
}

// ----- THE SLOG HANDLER -----------------------------------------------------

// NewSlogHandler returns a log/slog handler that records each record as a log event through the logger, with
// default_category for a record that has no category attribute. The level is not filtered by the handler.
func NewSlogHandler(logger Logger, default_category string) slog.Handler {
	return sdk.NewSlogHandler(logger, default_category)
}

// ----- PROFILES -------------------------------------------------------------

// Microservice returns the defaults for a long-running service: the delivery mode is best_effort.
func Microservice() Config { return infra_config.Microservice() }

// Cronjob returns the defaults for a short scheduled job: the delivery mode is retry.
func Cronjob() Config { return infra_config.Cronjob() }

// Worker returns the defaults for a background worker: the delivery mode is retry.
func Worker() Config { return infra_config.Worker() }

// Robotics returns the defaults for a robot: the delivery mode is durable, so it needs a spool directory and a spool.
func Robotics() Config { return infra_config.Robotics() }

// OfflineDevice returns the defaults for a device that is often offline: the delivery mode is durable, so it needs a
// spool directory and a spool.
func OfflineDevice() Config { return infra_config.OfflineDevice() }

// ----- OPTIONS --------------------------------------------------------------

// WithLogSink adds a destination for the log events. Several sinks of the same kind are allowed.
func WithLogSink(sink LogSinkPort) Option { return config_options.WithLogSink(sink) }

// WithEventSink adds a destination for the events that have no kind of their own.
func WithEventSink(sink EventSinkPort) Option { return config_options.WithEventSink(sink) }

// WithMetricsSink adds a destination for the metric events.
func WithMetricsSink(sink MetricsSinkPort) Option { return config_options.WithMetricsSink(sink) }

// WithTraceSink adds a destination for the trace events.
func WithTraceSink(sink TraceSinkPort) Option { return config_options.WithTraceSink(sink) }

// WithDeviceTelemetrySink adds a destination for the device telemetry events.
func WithDeviceTelemetrySink(sink DeviceTelemetrySinkPort) Option {
	return config_options.WithDeviceTelemetrySink(sink)
}

// WithAITelemetrySink adds a destination for the AI telemetry events.
func WithAITelemetrySink(sink AITelemetrySinkPort) Option {
	return config_options.WithAITelemetrySink(sink)
}

// WithLocalSpool sets the spool of the durable delivery mode.
func WithLocalSpool(spool LocalSpoolPort) Option { return config_options.WithLocalSpool(spool) }

// WithClock sets the clock, instead of the system clock.
func WithClock(clock ClockPort) Option { return config_options.WithClock(clock) }

// WithIDGenerator sets the identifier generator, instead of the UUIDv7 generator.
func WithIDGenerator(id_generator IDGeneratorPort) Option {
	return config_options.WithIDGenerator(id_generator)
}

// ----- VALUES ---------------------------------------------------------------

// NewEventAttributes returns the EventAttributes of the entries, or an error when a key or a value is not allowed.
func NewEventAttributes(entries map[string]any) (EventAttributes, error) {
	return value_objects.NewEventAttributes(entries)
}

// NewActorIdentity returns the ActorIdentity of the kind (user, service, system, agent or device, as in the contract),
// the id and the options. It returns an error when a field is not valid.
func NewActorIdentity(kind string, id string, options ...ActorOption) (ActorIdentity, error) {
	actor_kind, kind_error := value_objects.NewActorKind(kind)
	if kind_error != nil {
		return ActorIdentity{}, kind_error
	}
	return value_objects.NewActorIdentity(actor_kind, id, options...)
}

// WithActorType sets the type of the actor, for example llm.
func WithActorType(actor_type string) ActorOption { return value_objects.WithActorType(actor_type) }

// WithActorVersion sets the version of the actor.
func WithActorVersion(actor_version string) ActorOption {
	return value_objects.WithActorVersion(actor_version)
}

// NewEventID returns the EventID of the text, or an error when it is not a lowercase UUID of version 7.
func NewEventID(text string) (EventID, error) { return value_objects.NewEventID(text) }

// NewUTCInstant returns the UTCInstant of the time, which must be in UTC.
func NewUTCInstant(instant time.Time) (UTCInstant, error) {
	return value_objects.NewUTCInstant(instant)
}

// ----- CALL CONTEXT ---------------------------------------------------------

// NewCallContext returns the CallContext of the options, or an error that names each invalid field.
func NewCallContext(options ...CallContextOption) (CallContext, error) {
	return call_context.NewCallContext(options...)
}

// WithCallContext returns a copy of ctx that carries the call context; Emit attaches it to the event.
func WithCallContext(ctx context.Context, carried_call_context CallContext) context.Context {
	return call_context.WithCallContext(ctx, carried_call_context)
}

// FromContext returns the call context that ctx carries and whether there is one.
func FromContext(ctx context.Context) (CallContext, bool) { return call_context.FromContext(ctx) }

// NextIndentation returns a copy of ctx one layer deeper and true, or ctx and false when it carries no call context.
func NextIndentation(ctx context.Context) (context.Context, bool) {
	return call_context.NextIndentation(ctx)
}

// WithApplication sets the name of the application in a call context.
func WithApplication(application_name string) CallContextOption {
	return call_context.WithApplication(application_name)
}

// WithInstanceID sets the id of the running instance in a call context.
func WithInstanceID(instance_id string) CallContextOption {
	return call_context.WithInstanceID(instance_id)
}

// WithExecutionID sets the id of the execution in a call context.
func WithExecutionID(execution_id string) CallContextOption {
	return call_context.WithExecutionID(execution_id)
}

// WithCorrelationID sets the correlation id in a call context.
func WithCorrelationID(correlation_id string) CallContextOption {
	return call_context.WithCorrelationID(correlation_id)
}

// WithTraceID sets the W3C trace id in a call context.
func WithTraceID(trace_id string) CallContextOption { return call_context.WithTraceID(trace_id) }

// WithSpanID sets the W3C span id in a call context.
func WithSpanID(span_id string) CallContextOption { return call_context.WithSpanID(span_id) }
