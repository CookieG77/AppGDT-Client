// HTTP client for the AppGDT-Server API: every call made by the pages goes through it

package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// maxResponseSize protects the client from an unexpectedly huge response.
// It leaves room for an account export with many long notes.
const maxResponseSize = 32 << 20

type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a client for the API located at baseURL (without trailing slash).
// Every call is cancelled after timeout.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: timeout,
			// The API never redirects: a redirect is treated as a response
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Health checks that the API and its database answer.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", "", nil, nil)
}

// do sends a request to the API and decodes the answer.
//   - token: JWT sent in the Authorization header, empty for public routes
//   - in: body encoded as JSON, nil for no body
//   - out: pointer receiving the decoded JSON answer, nil to ignore it
//
// It returns an *APIError when the API answers with an error status,
// and an error wrapping ErrUnavailable when the API cannot be reached.
func (c *Client) do(ctx context.Context, method, path, token string, in, out any) error {
	resp, err := c.send(ctx, method, path, token, in)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		// Drain the body so the connection can be reused
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseSize))
		return nil
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(out); err != nil {
		return fmt.Errorf("decoding response of %s %s failed: %w", method, path, err)
	}
	return nil
}

// send builds and sends the request, and turns error statuses into *APIError.
// On success, the caller must close the response body.
func (c *Client) send(ctx context.Context, method, path, token string, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		payload, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("encoding request body failed: %w", err)
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// A request cancelled by the browser is not an API failure
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, ctx.Err()
		}
		slog.WarnContext(ctx, "api unreachable", "method", method, "path", path, "error", err)
		return nil, fmt.Errorf("%w: %s %s: %v", ErrUnavailable, method, path, err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	defer resp.Body.Close()
	return nil, readAPIError(resp)
}

// readAPIError decodes an error answer of the API.
// If the body does not follow the Error schema, a generic error is built
// from the status code so the caller always gets an *APIError.
func readAPIError(resp *http.Response) *APIError {
	apiErr := &APIError{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(apiErr); err != nil || apiErr.Code == "" {
		apiErr = &APIError{
			Code:    "UNEXPECTED_RESPONSE",
			Message: "Réponse inattendue du serveur.",
		}
	}
	apiErr.Status = resp.StatusCode

	if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
		apiErr.RetryAfter = time.Duration(seconds) * time.Second
	}
	return apiErr
}
