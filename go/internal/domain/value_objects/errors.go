package value_objects

import "errors"

// The sentinels of the value objects live in the domain root (domain/errors.go); each value object file aliases the
// ones it returns. This file holds only the composition of errors.

// ===== ERROR COMPOSITION ====================================================

// JoinErrors joins the non-nil errors into one, so errors.Is finds each of them. It returns nil when there is none.
func JoinErrors(errs ...error) error {
	return errors.Join(errs...)
}

// field_failure joins the sentinel of a field with the cause that a base value object reported. It returns nil when
// the cause is nil, so a constructor can collect the failures of every field and report them together.
func field_failure(field_sentinel error, cause error) error {
	if cause == nil {
		return nil
	}
	return JoinErrors(field_sentinel, cause)
}
