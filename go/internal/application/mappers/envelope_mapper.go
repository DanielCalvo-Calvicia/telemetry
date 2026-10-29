// Package mappers turns the validated value objects of the domain into the wire structs of the contract. It holds no
// business logic: it copies fields.
package mappers

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// ENVELOPE MAPPER
// =============================================================================

// The null-versus-empty rules of the envelope. They are the same in Go and in Python:
//
//   - data is NEVER null. A nil map becomes an empty non-nil map, and a top-level copy of the map of the caller is
//     kept, so a later change by the caller does not change the envelope. It serializes as {}.
//   - attributes is omitted when empty: the map is nil (the omitempty of the envelope drops it).
//   - actor and context are omitted when absent: the pointer is nil. A context with no field at all counts as absent.
//
// Nothing here is masked or limited yet. An envelope made with these functions must not be sent to any sink before
// the masking of T-03.4 (ADR-006).

// MapResource returns the wire resource of a resource identity. An optional field that is absent stays empty, and the
// omitempty of the contract struct drops it.
func MapResource(resource value_objects.ResourceIdentity) value_objects.Resource {
	wire_resource := value_objects.Resource{
		ServiceName: resource.ServiceName().String(),
		InstanceId:  resource.InstanceID().String(),
	}
	if environment, is_present := resource.Environment(); is_present {
		wire_resource.Environment = environment.String()
	}
	if service_version, is_present := resource.ServiceVersion(); is_present {
		wire_resource.ServiceVersion = service_version.String()
	}
	if host, is_present := resource.Host(); is_present {
		wire_resource.Host = host.String()
	}
	if time_zone, is_present := resource.TimeZone(); is_present {
		wire_resource.TimeZone = time_zone
	}
	return wire_resource
}

// MapActor returns the wire actor of an actor identity, or nil when there is none.
func MapActor(actor *value_objects.ActorIdentity) *value_objects.Actor {
	if actor == nil {
		return nil
	}
	wire_actor := value_objects.Actor{Kind: actor.Kind(), Id: actor.ID().String()}
	if actor_type, is_present := actor.Type(); is_present {
		wire_actor.Type = actor_type.String()
	}
	if actor_version, is_present := actor.Version(); is_present {
		wire_actor.Version = actor_version.String()
	}
	return &wire_actor
}

// MapContext returns the wire context of a correlation context, or nil when there is none or when it has no field at
// all.
func MapContext(correlation_context *value_objects.CorrelationContext) *value_objects.Context {
	if correlation_context == nil {
		return nil
	}
	var wire_context value_objects.Context
	has_field := false
	if trace_id, is_present := correlation_context.TraceID(); is_present {
		wire_context.TraceId, has_field = trace_id.String(), true
	}
	if span_id, is_present := correlation_context.SpanID(); is_present {
		wire_context.SpanId, has_field = span_id.String(), true
	}
	if execution_id, is_present := correlation_context.ExecutionID(); is_present {
		wire_context.ExecutionId, has_field = execution_id.String(), true
	}
	if correlation_id, is_present := correlation_context.CorrelationID(); is_present {
		wire_context.CorrelationId, has_field = correlation_id.String(), true
	}
	if !has_field {
		return nil
	}
	return &wire_context
}

// MapAttributes returns the wire attributes of event attributes: nil when there are none.
func MapAttributes(attributes value_objects.EventAttributes) value_objects.Attributes {
	if attributes.Len() == 0 {
		return nil
	}
	return value_objects.Attributes(attributes.Entries())
}

// CopyData returns a top-level copy of the data of the caller, and an empty non-nil map for a nil one.
func CopyData(data map[string]any) map[string]any {
	data_copy := make(map[string]any, len(data))
	for key, value := range data {
		data_copy[key] = value
	}
	return data_copy
}
