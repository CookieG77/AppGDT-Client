package session

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
)

// withCookies returns a request carrying the cookies set on rec.
func withCookies(rec *httptest.ResponseRecorder, method, body string) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestCSRFToken(t *testing.T) {
	m := NewManager(false)
	rec := httptest.NewRecorder()
	token := m.CSRFToken(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !validToken(token) {
		t.Fatalf("invalid token %q", token)
	}

	// The same token is kept while the cookie exists
	if again := m.CSRFToken(httptest.NewRecorder(), withCookies(rec, http.MethodGet, "")); again != token {
		t.Error("the token must be reused")
	}

	if !m.CheckCSRF(withCookies(rec, http.MethodPost, url.Values{CSRFField: {token}}.Encode())) {
		t.Error("the right token must be accepted")
	}
	if m.CheckCSRF(withCookies(rec, http.MethodPost, url.Values{CSRFField: {token + "x"}}.Encode())) {
		t.Error("a wrong token must be refused")
	}
	if m.CheckCSRF(withCookies(rec, http.MethodPost, "")) {
		t.Error("a missing token must be refused")
	}
	if m.CheckCSRF(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{CSRFField: {token}}.Encode()))) {
		t.Error("a token without its cookie must be refused")
	}
}

func TestCookieAttributes(t *testing.T) {
	for _, secure := range []bool{false, true} {
		m := NewManager(secure)
		rec := httptest.NewRecorder()
		m.SetToken(rec, apiclient.AuthToken{Token: "jwt", ExpiresIn: 3600})

		c := rec.Result().Cookies()[0]
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != 3600 || c.Secure != secure {
			t.Errorf("secure=%v: unexpected cookie %+v", secure, c)
		}
		if secure != strings.HasPrefix(c.Name, "__Host-") {
			t.Errorf("secure=%v: cookie name %q", secure, c.Name)
		}
	}
}

func TestFlashIsShownOnce(t *testing.T) {
	m := NewManager(false)
	rec := httptest.NewRecorder()
	m.SetFlash(rec, "success", "Note enregistrée & fermée")

	read := httptest.NewRecorder()
	flash := m.PopFlash(read, withCookies(rec, http.MethodGet, ""))
	if flash == nil || flash.Kind != "success" || flash.Message != "Note enregistrée & fermée" {
		t.Fatalf("unexpected flash %+v", flash)
	}
	if c := read.Result().Cookies(); len(c) != 1 || c[0].MaxAge >= 0 {
		t.Error("the flash cookie must be deleted once read")
	}

	// An unknown kind falls back to "info"
	rec = httptest.NewRecorder()
	m.SetFlash(rec, "<script>", "x")
	if f := m.PopFlash(httptest.NewRecorder(), withCookies(rec, http.MethodGet, "")); f.Kind != "info" {
		t.Errorf("kind = %q, want info", f.Kind)
	}
}
