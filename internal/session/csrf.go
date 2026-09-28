package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"
)

// CSRFField is the name of the hidden field carrying the token in forms.
const CSRFField = "csrf_token"

// csrfLifetime keeps the same token for a whole browsing session of a day,
// so that a form left open for a while can still be sent.
const csrfLifetime = 24 * time.Hour

// CSRFToken returns the token stored in the browser, creating it if needed.
//
// Protection used: "double submit cookie". The token is stored in an
// HttpOnly cookie and repeated in a hidden field of every form. Another site
// can make the browser send the cookie, but it can neither read it nor guess
// it, so it cannot put the right value in the form.
func (m *Manager) CSRFToken(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(m.csrfCookie); err == nil && validToken(c.Value) {
		return c.Value
	}

	buf := make([]byte, 32)
	// crypto/rand never fails on supported platforms (it panics otherwise)
	_, _ = rand.Read(buf)
	token := base64.RawURLEncoding.EncodeToString(buf)
	http.SetCookie(w, m.newCookie(m.csrfCookie, token, csrfLifetime))
	return token
}

// CheckCSRF reports whether the token sent with the form matches the cookie.
// The comparison takes a constant time, so it leaks nothing about the token.
func (m *Manager) CheckCSRF(r *http.Request) bool {
	c, err := r.Cookie(m.csrfCookie)
	if err != nil || !validToken(c.Value) {
		return false
	}
	sent := r.PostFormValue(CSRFField)
	return subtle.ConstantTimeCompare([]byte(sent), []byte(c.Value)) == 1
}

// validToken checks the format of a token read from a cookie.
func validToken(token string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}
