package external

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// PostEntry posts the JSON body to the endpoint. A transport failure is returned as it comes; a response with
// a status of 400 or above is returned as a StatusError.
func (client *Client) PostEntry(ctx context.Context, endpoint string, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.http_client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusBadRequest {
		return StatusError{StatusCode: response.StatusCode}
	}
	return nil
}
