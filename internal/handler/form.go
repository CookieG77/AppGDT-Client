// Values and errors of a submitted form, shared by every form page

package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
)

// Form holds what the user typed and the errors to show next to each field.
type Form struct {
	// Values typed by the user, sent back in the inputs (never passwords)
	Values map[string]string
	// Errors indexed by field name, shown under each label
	Errors map[string]string
	// Summary lists the field errors in the order of the form, for the
	// error summary at the top (each item links to its field)
	Summary []FieldMessage
	// Message is an error that does not belong to a single field
	Message string
}

type FieldMessage struct {
	Field   string
	Message string
}

// newForm reads the given fields from the submitted form. Values are trimmed,
// except the ones listed in raw (passwords must be sent exactly as typed).
func newForm(r *http.Request, fields []string, raw ...string) *Form {
	f := &Form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, name := range fields {
		value := r.PostFormValue(name)
		if !contains(raw, name) {
			value = strings.TrimSpace(value)
		}
		f.Values[name] = value
	}
	return f
}

// emptyForm is used to display a form for the first time.
func emptyForm() *Form {
	return &Form{Values: map[string]string{}, Errors: map[string]string{}}
}

func (f *Form) Get(name string) string { return f.Values[name] }

// AddError records an error on a field. Only the first error of a field is kept.
func (f *Form) AddError(field, message string) {
	if _, exists := f.Errors[field]; exists {
		return
	}
	f.Errors[field] = message
	f.Summary = append(f.Summary, FieldMessage{Field: field, Message: message})
}

// Required adds an error on each listed field left empty.
func (f *Form) Required(messages map[string]string, order ...string) {
	for _, field := range order {
		if f.Values[field] == "" {
			f.AddError(field, messages[field])
		}
	}
}

func (f *Form) HasErrors() bool {
	return len(f.Errors) > 0 || f.Message != ""
}

// applyAPIError copies the validation errors of the API on the form fields,
// following the order of the form. Errors on unknown fields become the
// general message.
func (f *Form) applyAPIError(apiErr *apiclient.APIError, order []string) {
	fieldErrors := apiErr.FieldErrors()
	for _, field := range order {
		if msg, ok := fieldErrors[field]; ok {
			f.AddError(field, msg)
			delete(fieldErrors, field)
		}
	}
	if len(fieldErrors) > 0 || len(apiErr.Details) == 0 {
		f.Message = apiErr.Message
	}
}

// handleAPIError fills the form with the error returned by the API and
// returns the status code to answer with.
func handleAPIError(r *http.Request, f *Form, err error, order []string) int {
	apiErr, isAPIErr := apiclient.AsAPIError(err)

	switch {
	case errors.Is(err, apiclient.ErrUnavailable):
		f.Message = "Le service est momentanément indisponible. Réessayez dans quelques instants."
		return http.StatusServiceUnavailable

	case isAPIErr && apiErr.Status == http.StatusBadRequest:
		f.applyAPIError(apiErr, order)
		return http.StatusUnprocessableEntity

	case isAPIErr && apiErr.Status == http.StatusTooManyRequests:
		f.Message = tooManyAttemptsMessage(apiErr)
		return http.StatusTooManyRequests

	case isAPIErr && apiErr.Status < 500:
		f.Message = apiErr.Message
		return apiErr.Status

	default:
		slog.ErrorContext(r.Context(), "unexpected API error", "error", err, "path", r.URL.Path)
		f.Message = "Une erreur inattendue est survenue. Réessayez dans quelques instants."
		return http.StatusInternalServerError
	}
}

// tooManyAttemptsMessage tells the user how long to wait, in minutes.
func tooManyAttemptsMessage(apiErr *apiclient.APIError) string {
	minutes := int(math.Ceil(apiErr.RetryAfter.Minutes()))
	switch {
	case minutes <= 0:
		return "Trop de tentatives échouées. Réessayez dans quelques minutes."
	case minutes == 1:
		return "Trop de tentatives échouées. Réessayez dans 1 minute."
	default:
		return fmt.Sprintf("Trop de tentatives échouées. Réessayez dans %d minutes.", minutes)
	}
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
