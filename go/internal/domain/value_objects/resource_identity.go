package value_objects

import (
	"regexp"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// ===== ResourceIdentity =====================================================

// ResourceIdentity says who produces the events: the service and the instance of it. service_name and instance_id
// are required. environment, service_version, host and time_zone are optional: absent is allowed, present but empty
// is not (the contract gives them a minimum length of 1). The time zone is a name for display; it is never used to
// compute a time (ADR-012). The wire struct of the contract is Resource (generated); this is its validated form.
type ResourceIdentity struct {
	service_name    base_value_objects.Identifier
	instance_id     base_value_objects.Identifier
	environment     optional_field[base_value_objects.Identifier]
	service_version optional_field[base_value_objects.Identifier]
	host            optional_field[base_value_objects.Identifier]
	time_zone       optional_field[string]
}

// ResourceOption sets an optional field of a ResourceIdentity. A field that is set is present, even when its text is
// empty; the constructor then rejects it.
type ResourceOption func(resource *ResourceIdentity)

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrResourceServiceNameInvalid is domain.ErrResourceServiceNameInvalid.
	ErrResourceServiceNameInvalid = domain.ErrResourceServiceNameInvalid
	// ErrResourceInstanceIDInvalid is domain.ErrResourceInstanceIDInvalid.
	ErrResourceInstanceIDInvalid = domain.ErrResourceInstanceIDInvalid
	// ErrResourceEnvironmentInvalid is domain.ErrResourceEnvironmentInvalid.
	ErrResourceEnvironmentInvalid = domain.ErrResourceEnvironmentInvalid
	// ErrResourceServiceVersionInvalid is domain.ErrResourceServiceVersionInvalid.
	ErrResourceServiceVersionInvalid = domain.ErrResourceServiceVersionInvalid
	// ErrResourceHostInvalid is domain.ErrResourceHostInvalid.
	ErrResourceHostInvalid = domain.ErrResourceHostInvalid
	// ErrTimeZoneInvalid is domain.ErrTimeZoneInvalid.
	ErrTimeZoneInvalid = domain.ErrTimeZoneInvalid
)

// ----- CONSTANTS ------------------------------------------------------------

// time_zone_pattern is the shape of the time_zone (domain.TimeZonePattern).
var time_zone_pattern = regexp.MustCompile(domain.TimeZonePattern)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewResourceIdentity returns the ResourceIdentity of the required fields and the options. When a field is invalid
// it returns the sentinel of each invalid field (for example ErrResourceServiceNameInvalid) joined with the cause of
// the base value object, so errors.Is finds both.
func NewResourceIdentity(service_name string, instance_id string, options ...ResourceOption) (ResourceIdentity, error) {
	resource := ResourceIdentity{
		service_name: base_value_objects.Identifier(service_name),
		instance_id:  base_value_objects.Identifier(instance_id),
	}
	for _, option := range options {
		option(&resource)
	}
	identifier_validator := base_value_objects.ValidateIdentifier
	validation_error := JoinErrors(
		field_failure(ErrResourceServiceNameInvalid, identifier_validator(service_name)),
		field_failure(ErrResourceInstanceIDInvalid, identifier_validator(instance_id)),
		field_failure(ErrResourceEnvironmentInvalid, resource.environment.validate_if_present(identifier_validator)),
		field_failure(ErrResourceServiceVersionInvalid, resource.service_version.validate_if_present(identifier_validator)),
		field_failure(ErrResourceHostInvalid, resource.host.validate_if_present(identifier_validator)),
		resource.time_zone.validate_if_present(ValidateTimeZone),
	)
	if validation_error != nil {
		return ResourceIdentity{}, validation_error
	}
	return resource, nil
}

// WithEnvironment sets the environment of the resource, for example staging.
func WithEnvironment(environment string) ResourceOption {
	return func(resource *ResourceIdentity) {
		resource.environment = present_field(base_value_objects.Identifier(environment))
	}
}

// WithServiceVersion sets the version of the service.
func WithServiceVersion(service_version string) ResourceOption {
	return func(resource *ResourceIdentity) {
		resource.service_version = present_field(base_value_objects.Identifier(service_version))
	}
}

// WithHost sets the name of the host.
func WithHost(host string) ResourceOption {
	return func(resource *ResourceIdentity) {
		resource.host = present_field(base_value_objects.Identifier(host))
	}
}

// WithTimeZone sets the time zone name of the resource, for display only.
func WithTimeZone(time_zone string) ResourceOption {
	return func(resource *ResourceIdentity) {
		resource.time_zone = present_field(time_zone)
	}
}

// ----- METHODS --------------------------------------------------------------

// ServiceName returns the name of the service.
func (resource ResourceIdentity) ServiceName() base_value_objects.Identifier {
	return resource.service_name
}

// InstanceID returns the id of the instance of the service.
func (resource ResourceIdentity) InstanceID() base_value_objects.Identifier {
	return resource.instance_id
}

// Environment returns the environment and whether it is present.
func (resource ResourceIdentity) Environment() (base_value_objects.Identifier, bool) {
	return resource.environment.get()
}

// ServiceVersion returns the version of the service and whether it is present.
func (resource ResourceIdentity) ServiceVersion() (base_value_objects.Identifier, bool) {
	return resource.service_version.get()
}

// Host returns the name of the host and whether it is present.
func (resource ResourceIdentity) Host() (base_value_objects.Identifier, bool) {
	return resource.host.get()
}

// TimeZone returns the time zone name and whether it is present. It is for display and never computes a time.
func (resource ResourceIdentity) TimeZone() (string, bool) {
	return resource.time_zone.get()
}

// ----- VALIDATORS -----------------------------------------------------------

// ValidateTimeZone returns nil when the text has the shape of an IANA time zone name (domain.TimeZonePattern), and
// ErrTimeZoneInvalid otherwise. It does not look the name up in a database.
func ValidateTimeZone(text string) error {
	if !time_zone_pattern.MatchString(text) {
		return ErrTimeZoneInvalid
	}
	return nil
}
