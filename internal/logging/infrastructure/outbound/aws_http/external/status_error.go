package external

import "strconv"

// StatusError is the root cause reported when the AWS log receiver answers with a status of 400 or above.
type StatusError struct {
	StatusCode int
}

// Error returns the text of the failure with the status code the receiver answered.
func (status_error StatusError) Error() string {
	return "AWS log receiver answered with status " + strconv.Itoa(status_error.StatusCode)
}
