package external

import (
	"net/http"
	"time"
)

// Client posts log entries to the log receiver; it is the only holder of an HTTP client in the codebase.
type Client struct {
	http_client *http.Client
}

// NewClient creates a Client whose requests are abandoned after the timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{http_client: &http.Client{Timeout: timeout}}
}
