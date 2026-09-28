// Cookies kept in the browser (JWT, flash message, CSRF token) and the
// per-request data derived from them

package session

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
)

// Manager reads and writes the cookies of the client.
//
// Every cookie is HttpOnly (unreadable by JavaScript) and SameSite=Lax (not
// sent with cross-site POST requests). When HTTPS is enabled, cookies are
// also Secure and use the "__Host-" prefix, which makes the browser refuse
// them unless they are Secure, bound to this exact host and to path "/".
type Manager struct {
	secure      bool
	tokenCookie string
	flashCookie string
	csrfCookie  string
}

func NewManager(secure bool) *Manager {
	prefix := ""
	if secure {
		prefix = "__Host-"
	}
	return &Manager{
		secure:      secure,
		tokenCookie: prefix + "gdt_session",
		flashCookie: prefix + "gdt_flash",
		csrfCookie:  prefix + "gdt_csrf",
	}
}

func (m *Manager) newCookie(name, value string, maxAge time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (m *Manager) clear(w http.ResponseWriter, name string) {
	c := m.newCookie(name, "", 0)
	c.MaxAge = -1
	http.SetCookie(w, c)
}

// --- JWT -------------------------------------------------------------------

// SetToken stores the JWT until it expires.
func (m *Manager) SetToken(w http.ResponseWriter, token apiclient.AuthToken) {
	http.SetCookie(w, m.newCookie(m.tokenCookie, token.Token, token.Lifetime()))
}

// Token returns the stored JWT, or "" if there is none.
func (m *Manager) Token(r *http.Request) string {
	c, err := r.Cookie(m.tokenCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// ClearToken removes the JWT (logout, expired or rejected token).
func (m *Manager) ClearToken(w http.ResponseWriter) {
	m.clear(w, m.tokenCookie)
}

// --- Flash messages ----------------------------------------------------------

// Flash is a one-time message shown on the next page (Post/Redirect/Get).
type Flash struct {
	// Kind is "success" or "info"
	Kind    string
	Message string
}

// SetFlash stores a message to show on the next page.
func (m *Manager) SetFlash(w http.ResponseWriter, kind, message string) {
	value := url.Values{"k": {kind}, "m": {message}}.Encode()
	http.SetCookie(w, m.newCookie(m.flashCookie, value, time.Minute))
}

// PopFlash returns the pending message, if any, and deletes it.
func (m *Manager) PopFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie(m.flashCookie)
	if err != nil {
		return nil
	}
	m.clear(w, m.flashCookie)

	values, err := url.ParseQuery(c.Value)
	if err != nil || values.Get("m") == "" {
		return nil
	}
	kind := values.Get("k")
	if kind != "success" {
		kind = "info"
	}
	return &Flash{Kind: kind, Message: values.Get("m")}
}

// --- Request context -----------------------------------------------------------

type contextKey int

const (
	userKey contextKey = iota
	flashKey
	csrfKey
	unavailableKey
)

// WithAPIUnavailable records that the session could not be checked because
// the API did not answer: the user may be logged in, but we cannot know.
func WithAPIUnavailable(ctx context.Context) context.Context {
	return context.WithValue(ctx, unavailableKey, true)
}

func APIUnavailable(ctx context.Context) bool {
	down, _ := ctx.Value(unavailableKey).(bool)
	return down
}

// Current is the authenticated user of the request and their JWT.
type Current struct {
	User  *apiclient.User
	Token string
}

func WithCurrent(ctx context.Context, c *Current) context.Context {
	return context.WithValue(ctx, userKey, c)
}

// CurrentFrom returns the authenticated user, or nil for a visitor.
func CurrentFrom(ctx context.Context) *Current {
	c, _ := ctx.Value(userKey).(*Current)
	return c
}

func WithFlash(ctx context.Context, f *Flash) context.Context {
	return context.WithValue(ctx, flashKey, f)
}

func FlashFrom(ctx context.Context) *Flash {
	f, _ := ctx.Value(flashKey).(*Flash)
	return f
}

func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey, token)
}

// CSRFTokenFrom returns the token to put in every form of the page.
func CSRFTokenFrom(ctx context.Context) string {
	t, _ := ctx.Value(csrfKey).(string)
	return t
}
