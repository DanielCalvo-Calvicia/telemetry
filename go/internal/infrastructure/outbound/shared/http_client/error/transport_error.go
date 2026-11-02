// Package error translates the failures of the transport of the shared HTTP client to fixed reasons. The text of a
// transport error (it holds the URL, which may carry a secret in its query) never leaves this package: only the reason
// is text, the cause is kept for errors.Is and errors.As.
package error

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
)

// =============================================================================
// TRANSPORT ERROR
// =============================================================================

const (
	// ReasonTimeout is the reason of a request that did not end in time.
	ReasonTimeout = "timeout"
	// ReasonTLSVerificationFailed is the reason of a certificate that was refused.
	ReasonTLSVerificationFailed = "tls_verification_failed"
	// ReasonNetworkError is the reason of any other failure at the level of the connection.
	ReasonNetworkError = "network_error"
	// ReasonResponseUnreadable is the reason of a response whose body could not be read after the status.
	ReasonResponseUnreadable = "response_unreadable"
)

// TransportError is a failure of the transport with a fixed reason. Its text is the reason and nothing else; the cause
// is reachable with errors.Is and errors.As, never as text.
type TransportError struct {
	reason string
	cause  error
}

// Error returns the fixed reason.
func (transport_error *TransportError) Error() string {
	return transport_error.reason
}

// Unwrap returns the cause, for errors.Is and errors.As.
func (transport_error *TransportError) Unwrap() error {
	return transport_error.cause
}

// Reason returns the fixed reason: one of the Reason constants.
func (transport_error *TransportError) Reason() string {
	return transport_error.reason
}

// MapTransportError translates a failure of the connection (the request did not give a response) to a *TransportError:
// timeout, tls_verification_failed or network_error.
func MapTransportError(cause error) *TransportError {
	return &TransportError{reason: reason_of(cause), cause: cause}
}

// MapReadError translates a failure to read the body of a response to a *TransportError with the reason
// response_unreadable.
func MapReadError(cause error) *TransportError {
	return &TransportError{reason: ReasonResponseUnreadable, cause: cause}
}

// ----- HELPERS --------------------------------------------------------------

// timeout_error is what a net.Error offers; net is not imported here because it is an I/O package.
type timeout_error interface {
	Timeout() bool
}

// reason_of returns the fixed reason of a failure of the connection.
func reason_of(cause error) string {
	var timeout timeout_error
	if errors.Is(cause, context.DeadlineExceeded) || (errors.As(cause, &timeout) && timeout.Timeout()) {
		return ReasonTimeout
	}
	var verification_error *tls.CertificateVerificationError
	var unknown_authority x509.UnknownAuthorityError
	var invalid_certificate x509.CertificateInvalidError
	var hostname_error x509.HostnameError
	if errors.As(cause, &verification_error) || errors.As(cause, &unknown_authority) ||
		errors.As(cause, &invalid_certificate) || errors.As(cause, &hostname_error) {
		return ReasonTLSVerificationFailed
	}
	return ReasonNetworkError
}
