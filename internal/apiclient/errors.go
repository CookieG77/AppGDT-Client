package apiclient

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// ErrUnavailable is returned when the API cannot be reached
// (connection refused, timeout, DNS error…).
var ErrUnavailable = errors.New("api unavailable")

// FieldError is the validation error of a single field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is an error answered by the API, following its Error schema.
type APIError struct {
	Status  int          `json:"-"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
	// RetryAfter is set from the Retry-After header on 429 responses
	RetryAfter time.Duration `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d %s: %s", e.Status, e.Code, e.Message)
}

// FieldErrors returns the validation errors indexed by field name,
// so that templates can show each message next to its input.
func (e *APIError) FieldErrors() map[string]string {
	fields := make(map[string]string, len(e.Details))
	for _, d := range e.Details {
		// Keep the first message when a field has several errors
		if _, exists := fields[d.Field]; !exists {
			fields[d.Field] = d.Message
		}
	}
	return fields
}

// AsAPIError returns the APIError wrapped in err, if any.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	return apiErr, ok
}

// hasStatus reports whether err is an APIError with the given status.
func hasStatus(err error, status int) bool {
	apiErr, ok := AsAPIError(err)
	return ok && apiErr.Status == status
}

// IsUnauthorized reports whether the token is missing, invalid or expired.
func IsUnauthorized(err error) bool { return hasStatus(err, http.StatusUnauthorized) }

// IsNotFound reports whether the resource does not exist or belongs to another user.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsValidation reports whether the API rejected the sent data.
func IsValidation(err error) bool { return hasStatus(err, http.StatusBadRequest) }
