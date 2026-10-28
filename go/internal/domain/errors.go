package domain

import "errors"

// The sentinels of the domain operations. Each one is wrapped in a ClassifiedError, which carries the code of the
// catalog (contracts/errors/catalog.yaml). The full error model arrives with T-03.7; these are small on purpose and
// easy to move.
var (
	// ErrNumberOutOfRange is returned for a number that has no exact canonical form: NaN, an infinity or an integer
	// beyond the range of exact integers (2^53 - 1).
	ErrNumberOutOfRange = errors.New("the number is outside the range of the canonical form")
	// ErrUnsupportedValue is returned for a value that has no canonical JSON form, such as a channel or a string that
	// is not valid UTF-8.
	ErrUnsupportedValue = errors.New("the value has no canonical JSON form")
	// ErrNonUTCTime is returned for an instant that is not in UTC or is outside the years 0000 to 9999.
	ErrNonUTCTime = errors.New("the instant is not a UTC time of the format of the contract")
)

// ClassifiedError is an error with the code of the catalog of error codes. It keeps the cause, so errors.Is finds the
// sentinel and errors.As finds the code.
type ClassifiedError struct {
	// Code is a code of contracts/errors/catalog.yaml (see the ErrorCode constants of value_objects).
	Code string
	// Err is the cause: a sentinel joined with the detail.
	Err error
}

// Error returns the code and the message of the cause.
func (classified *ClassifiedError) Error() string {
	return classified.Code + ": " + classified.Err.Error()
}

// Unwrap returns the cause, so errors.Is and errors.As reach the sentinel.
func (classified *ClassifiedError) Unwrap() error {
	return classified.Err
}

// NewClassifiedError builds a ClassifiedError from a code, a sentinel and an optional detail. The sentinel and the
// detail are joined, so both stay reachable.
func NewClassifiedError(code string, sentinel error, detail error) error {
	if detail == nil {
		return &ClassifiedError{Code: code, Err: sentinel}
	}
	return &ClassifiedError{Code: code, Err: errors.Join(sentinel, detail)}
}
