// Package external holds the only calls of the shared HTTP client to the network: one request with net/http.
package external

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

// =============================================================================
// POST REQUEST
// =============================================================================

// MaxResponseBytesRead is how much of the body of a response is read (and discarded): 64 KiB.
const MaxResponseBytesRead = 64 * 1024

// Request is one request to send. The header of the authentication has its name and value apart; an empty name means
// no authentication.
type Request struct {
	URL                 string
	Method              string
	TimeoutMilliseconds int
	SkipTLSVerification bool
	AuthHeaderName      string
	AuthHeaderValue     string
	Compressed          bool
	Body                []byte
}

// Response is what came back. When the request failed before a response, TransportErr holds the raw cause and the
// other fields are empty. When the body could not be read after the status, BodyReadErr holds the cause and the status
// and the header are still set. The raw errors hold the URL: only the error package turns them into a reason, nobody
// prints them.
type Response struct {
	StatusCode   int
	RetryAfter   string
	TransportErr error
	BodyReadErr  error
}

// PostRequest sends the request and returns the status, the value of the Retry-After header and any failure. The TLS
// certificate is verified unless the request says otherwise, a redirect is never followed (the 3xx is the answer), the
// time of the whole exchange is limited to the timeout and the body of the response is read up to 64 KiB and
// discarded.
func PostRequest(ctx context.Context, request Request) Response {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: request.SkipTLSVerification, MinVersion: tls.VersionTLS12},
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	http_client := &http.Client{
		Timeout:   time.Duration(request.TimeoutMilliseconds) * time.Millisecond,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	http_request, build_error := http.NewRequestWithContext(ctx, request.Method, request.URL, bytes.NewReader(request.Body))
	if build_error != nil {
		return Response{TransportErr: build_error}
	}
	http_request.Header.Set("Content-Type", "application/json")
	if request.Compressed {
		http_request.Header.Set("Content-Encoding", "gzip")
	}
	if request.AuthHeaderName != "" {
		http_request.Header.Set(request.AuthHeaderName, request.AuthHeaderValue)
	}

	http_response, send_error := http_client.Do(http_request)
	if send_error != nil {
		return Response{TransportErr: send_error}
	}
	defer http_response.Body.Close()
	response := Response{StatusCode: http_response.StatusCode, RetryAfter: http_response.Header.Get("Retry-After")}
	if _, read_error := io.Copy(io.Discard, io.LimitReader(http_response.Body, MaxResponseBytesRead)); read_error != nil {
		response.BodyReadErr = read_error
	}
	return response
}
