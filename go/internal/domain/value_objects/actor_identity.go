package value_objects

import (
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain"
	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/base_value_objects"
)

// ===== ActorIdentity ========================================================

// ActorIdentity says who or what acted: a kind from the contract and an id. type and version are optional: absent is
// allowed, present but empty is not. The wire struct of the contract is Actor (generated); this is its validated form.
type ActorIdentity struct {
	kind    ActorKind
	id      base_value_objects.Identifier
	type_   optional_field[base_value_objects.Identifier]
	version optional_field[base_value_objects.Identifier]
}

// ActorOption sets an optional field of an ActorIdentity. A field that is set is present, even when its text is empty;
// the constructor then rejects it.
type ActorOption func(actor *ActorIdentity)

// ----- ERRORS ---------------------------------------------------------------

var (
	// ErrActorIDInvalid is domain.ErrActorIDInvalid.
	ErrActorIDInvalid = domain.ErrActorIDInvalid
	// ErrActorTypeInvalid is domain.ErrActorTypeInvalid.
	ErrActorTypeInvalid = domain.ErrActorTypeInvalid
	// ErrActorVersionInvalid is domain.ErrActorVersionInvalid.
	ErrActorVersionInvalid = domain.ErrActorVersionInvalid
)

// ----- CONSTRUCTORS ---------------------------------------------------------

// NewActorIdentity returns the ActorIdentity of the kind, the id and the options. When a field is invalid it returns
// the sentinel of each invalid field (ErrActorKindInvalid, ErrActorIDInvalid, ...); the ones of the identifiers are
// joined with the cause of the base value object, so errors.Is finds both.
func NewActorIdentity(kind ActorKind, id string, options ...ActorOption) (ActorIdentity, error) {
	actor := ActorIdentity{kind: kind, id: base_value_objects.Identifier(id)}
	for _, option := range options {
		option(&actor)
	}
	identifier_validator := base_value_objects.ValidateIdentifier
	validation_error := JoinErrors(
		ValidateActorKind(string(kind)),
		field_failure(ErrActorIDInvalid, identifier_validator(id)),
		field_failure(ErrActorTypeInvalid, actor.type_.validate_if_present(identifier_validator)),
		field_failure(ErrActorVersionInvalid, actor.version.validate_if_present(identifier_validator)),
	)
	if validation_error != nil {
		return ActorIdentity{}, validation_error
	}
	return actor, nil
}

// WithActorType sets the type of the actor, for example llm.
func WithActorType(actor_type string) ActorOption {
	return func(actor *ActorIdentity) {
		actor.type_ = present_field(base_value_objects.Identifier(actor_type))
	}
}

// WithActorVersion sets the version of the actor.
func WithActorVersion(actor_version string) ActorOption {
	return func(actor *ActorIdentity) {
		actor.version = present_field(base_value_objects.Identifier(actor_version))
	}
}

// ----- METHODS --------------------------------------------------------------

// Kind returns the kind of the actor.
func (actor ActorIdentity) Kind() ActorKind {
	return actor.kind
}

// ID returns the id of the actor.
func (actor ActorIdentity) ID() base_value_objects.Identifier {
	return actor.id
}

// Type returns the type of the actor and whether it is present.
func (actor ActorIdentity) Type() (base_value_objects.Identifier, bool) {
	return actor.type_.get()
}

// Version returns the version of the actor and whether it is present.
func (actor ActorIdentity) Version() (base_value_objects.Identifier, bool) {
	return actor.version.get()
}
