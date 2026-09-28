package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient starts a fake API answering with handler.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New(srv.URL, 2*time.Second)
}

func TestLoginSendsJSONAndDecodesToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/login" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("public route must not send a token, got %q", got)
		}
		var in LoginInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Email != "a@b.c" || in.Password != "secret" {
			t.Errorf("unexpected body %+v (err %v)", in, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"jwt","tokenType":"Bearer","expiresIn":3600}`))
	})

	token, err := c.Login(context.Background(), LoginInput{Email: "a@b.c", Password: "secret"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token.Token != "jwt" || token.Lifetime() != time.Hour {
		t.Errorf("unexpected token %+v", token)
	}
}

func TestProtectedRouteSendsBearerToken(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jwt" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/spaces/42/notes" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":1,"spaceId":42,"title":"t","content":"c","status":"in_progress"}]`))
	})

	notes, err := c.ListNotes(context.Background(), "jwt", 42)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].Status != StatusInProgress {
		t.Errorf("unexpected notes %+v", notes)
	}
}

func TestValidationErrorIsDecoded(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"VALIDATION_ERROR","message":"invalide","details":[` +
			`{"field":"name","message":"Le nom est obligatoire."},` +
			`{"field":"name","message":"second message"}]}`))
	})

	_, err := c.CreateSpace(context.Background(), "jwt", SpaceInput{})
	if !IsValidation(err) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	apiErr, _ := AsAPIError(err)
	if got := apiErr.FieldErrors()["name"]; got != "Le nom est obligatoire." {
		t.Errorf("FieldErrors()[name] = %q", got)
	}
}

func TestStatusHelpersAndRetryAfter(t *testing.T) {
	status := http.StatusUnauthorized
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "30")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"code":"X","message":"m"}`))
	})
	ctx := context.Background()

	if _, err := c.Me(ctx, "expired"); !IsUnauthorized(err) {
		t.Errorf("expected unauthorized, got %v", err)
	}

	status = http.StatusNotFound
	if _, err := c.GetNote(ctx, "jwt", 7); !IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}

	status = http.StatusTooManyRequests
	_, err := c.Login(ctx, LoginInput{})
	apiErr, ok := AsAPIError(err)
	if !ok || apiErr.RetryAfter != 30*time.Second {
		t.Errorf("expected RetryAfter 30s, got %v", err)
	}
}

func TestNonJSONErrorStillGivesAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})

	err := c.Health(context.Background())
	apiErr, ok := AsAPIError(err)
	if !ok || apiErr.Status != http.StatusBadGateway || apiErr.Code != "UNEXPECTED_RESPONSE" {
		t.Errorf("unexpected error %v", err)
	}
}

func TestDeleteWithNoContent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/spaces/3" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteSpace(context.Background(), "jwt", 3); err != nil {
		t.Errorf("DeleteSpace: %v", err)
	}
}

func TestUnreachableAPI(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listens on this address anymore

	c := New(srv.URL, time.Second)
	if err := c.Health(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable, got %v", err)
	}
}

func TestExportAccountKeepsFilename(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="gdt-export-2026-09-28.json"`)
		_, _ = w.Write([]byte(`{"user":{}}`))
	})

	export, err := c.ExportAccount(context.Background(), "jwt")
	if err != nil {
		t.Fatalf("ExportAccount: %v", err)
	}
	if export.Filename != "gdt-export-2026-09-28.json" || string(export.Body) != `{"user":{}}` {
		t.Errorf("unexpected export %+v", export)
	}
}
