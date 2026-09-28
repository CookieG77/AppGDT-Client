package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
	"github.com/CookieG77/AppGDT-Client/web"
)

// fakeAPI imitates the routes of AppGDT-Server used by the pages.
type fakeAPI struct {
	calls atomic.Int32
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	writeJSON := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	apiError := func(status int, code, msg string) {
		writeJSON(status, map[string]string{"code": code, "message": msg})
	}
	authorized := r.Header.Get("Authorization") == "Bearer valid-token"

	switch r.Method + " " + r.URL.Path {
	case "POST /auth/login":
		var in apiclient.LoginInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch {
		case in.Email == "locked@test.fr":
			w.Header().Set("Retry-After", "120")
			apiError(http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Trop de tentatives.")
		case in.Password == "password123":
			writeJSON(http.StatusOK, apiclient.AuthToken{Token: "valid-token", TokenType: "Bearer", ExpiresIn: 3600})
		default:
			apiError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email ou mot de passe incorrect.")
		}
	case "POST /auth/register":
		var in apiclient.RegisterInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Email == "taken@test.fr" {
			apiError(http.StatusConflict, "EMAIL_ALREADY_USED", "Cette adresse email est déjà utilisée.")
			return
		}
		writeJSON(http.StatusCreated, apiclient.User{ID: 2, Email: in.Email, Username: in.Username})
	case "GET /users/me":
		if !authorized {
			apiError(http.StatusUnauthorized, "UNAUTHORIZED", "Authentification requise.")
			return
		}
		writeJSON(http.StatusOK, apiclient.User{ID: 1, Email: "ok@test.fr", Username: "Maxime"})
	case "GET /spaces":
		if !authorized {
			apiError(http.StatusUnauthorized, "UNAUTHORIZED", "Authentification requise.")
			return
		}
		writeJSON(http.StatusOK, []apiclient.Space{{ID: 1, Name: "Devoirs", Description: "École"}})
	default:
		apiError(http.StatusNotFound, "ROUTE_NOT_FOUND", "Route inconnue.")
	}
}

type testClient struct {
	t    *testing.T
	base string
	http *http.Client
	api  *fakeAPI
}

// newTestClient starts the client on top of a fake API, with a browser-like
// HTTP client that keeps cookies and does not follow redirects.
func newTestClient(t *testing.T) *testClient {
	t.Helper()

	api := &fakeAPI{}
	apiSrv := httptest.NewServer(api)
	t.Cleanup(apiSrv.Close)

	renderer, err := view.NewRenderer(web.FS)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	staticFS, _ := fs.Sub(web.FS, "static")
	apiClient := apiclient.New(apiSrv.URL, 2*time.Second)
	sessions := session.NewManager(false)

	srv := New("", Handlers{
		Page:  handler.NewPageHandler(renderer),
		Auth:  handler.NewAuthHandler(apiClient, renderer, sessions),
		Space: handler.NewSpaceHandler(apiClient, renderer, sessions),
	}, Sessions{Manager: sessions, API: apiClient}, staticFS, Options{})

	clientSrv := httptest.NewServer(srv.Handler)
	t.Cleanup(clientSrv.Close)

	jar, _ := cookiejar.New(nil)
	return &testClient{
		t:    t,
		base: clientSrv.URL,
		api:  api,
		http: &http.Client{
			Jar: jar,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *testClient) get(path string) (*http.Response, string) {
	c.t.Helper()
	resp, err := c.http.Get(c.base + path)
	if err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
	return resp, readBody(c.t, resp)
}

func (c *testClient) post(path string, form url.Values, headers ...string) (*http.Response, string) {
	c.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, c.base+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("POST %s: %v", path, err)
	}
	return resp, readBody(c.t, resp)
}

var csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

// csrfToken loads a page and returns the CSRF token of its forms.
func (c *testClient) csrfToken(path string) string {
	c.t.Helper()
	_, body := c.get(path)
	m := csrfPattern.FindStringSubmatch(body)
	if m == nil {
		c.t.Fatalf("no CSRF token in %s", path)
	}
	return m[1]
}

func (c *testClient) login() {
	c.t.Helper()
	token := c.csrfToken("/login")
	resp, _ := c.post("/login", url.Values{"csrf_token": {token}, "email": {"ok@test.fr"}, "password": {"password123"}})
	if resp.StatusCode != http.StatusSeeOther {
		c.t.Fatalf("login failed with status %d", resp.StatusCode)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

func expectStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func expectContains(t *testing.T, body string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(body, p) {
			t.Errorf("page does not contain %q", p)
		}
	}
}

// --- Tests -------------------------------------------------------------------------

func TestHomeIsPublic(t *testing.T) {
	c := newTestClient(t)
	resp, body := c.get("/")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "Vos notes, rangées", `href="/register"`)
}

func TestProtectedPageRedirectsToLogin(t *testing.T) {
	c := newTestClient(t)
	resp, _ := c.get("/spaces")
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/login?next=%2Fspaces" {
		t.Errorf("Location = %q", got)
	}
}

func TestLoginLogoutFlow(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login?next=/spaces")

	resp, _ := c.post("/login", url.Values{
		"csrf_token": {token}, "next": {"/spaces"},
		"email": {"ok@test.fr"}, "password": {"password123"},
	})
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/spaces" {
		t.Errorf("Location = %q, want /spaces", got)
	}
	var sessionCookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "gdt_session" {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie must be HttpOnly and SameSite=Lax, got %+v", sessionCookie)
	}

	resp, body := c.get("/spaces")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "Devoirs", "Maxime", `action="/logout"`)
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	// Logged-in users do not see the login page
	resp, _ = c.get("/login")
	expectStatus(t, resp, http.StatusSeeOther)

	resp, _ = c.post("/logout", url.Values{"csrf_token": {token}})
	expectStatus(t, resp, http.StatusSeeOther)
	resp, body = c.get("/")
	expectContains(t, body, "Vous êtes déconnecté")

	resp, _ = c.get("/spaces")
	expectStatus(t, resp, http.StatusSeeOther)
}

func TestFormWithoutCSRFTokenIsRefused(t *testing.T) {
	c := newTestClient(t)
	c.csrfToken("/login") // the browser has a CSRF cookie…

	// …but the form does not carry the token, as in a forged request
	resp, _ := c.post("/login", url.Values{"email": {"ok@test.fr"}, "password": {"password123"}})
	expectStatus(t, resp, http.StatusForbidden)
	if n := c.api.calls.Load(); n != 0 {
		t.Errorf("the API must not be called, got %d calls", n)
	}
}

func TestCrossSiteFormIsRefused(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login")
	resp, _ := c.post("/login",
		url.Values{"csrf_token": {token}, "email": {"ok@test.fr"}, "password": {"password123"}},
		"Sec-Fetch-Site", "cross-site")
	expectStatus(t, resp, http.StatusForbidden)
}

func TestLoginWithWrongPassword(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login")
	resp, body := c.post("/login", url.Values{"csrf_token": {token}, "email": {"ok@test.fr"}, "password": {"wrong-password"}})
	expectStatus(t, resp, http.StatusUnauthorized)
	expectContains(t, body,
		"<title>Erreur : Connexion · GDT</title>",
		"Email ou mot de passe incorrect.",
		`role="alert"`,
		`value="ok@test.fr"`, // the email is kept…
	)
	if strings.Contains(body, "wrong-password") {
		t.Error("the password must never be sent back in the page")
	}
}

func TestLoginRequiredFields(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login")
	resp, body := c.post("/login", url.Values{"csrf_token": {token}})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body,
		"Le formulaire contient 2 erreurs",
		`href="#field-email"`,
		`aria-invalid="true"`,
		`aria-describedby="field-email-error"`,
	)
	if n := c.api.calls.Load(); n != 0 {
		t.Errorf("the API must not be called for an incomplete form, got %d calls", n)
	}
}

func TestLoginTooManyAttempts(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login")
	resp, body := c.post("/login", url.Values{"csrf_token": {token}, "email": {"locked@test.fr"}, "password": {"password123"}})
	expectStatus(t, resp, http.StatusTooManyRequests)
	expectContains(t, body, "Réessayez dans 2 minutes.")
}

func TestRegisterChecksPasswordsBeforeCallingAPI(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/register")
	resp, body := c.post("/register", url.Values{
		"csrf_token": {token}, "username": {"Max"}, "email": {"max@test.fr"},
		"password": {"court"}, "password_confirm": {"autre"},
	})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body,
		"Le formulaire contient 2 erreurs",
		"au moins 8 caractères",
		"ne correspondent pas",
		`value="Max"`,
	)
	if n := c.api.calls.Load(); n != 0 {
		t.Errorf("the API must not be called, got %d calls", n)
	}
}

func TestRegisterWithUsedEmail(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/register")
	resp, body := c.post("/register", url.Values{
		"csrf_token": {token}, "username": {"Max"}, "email": {"taken@test.fr"},
		"password": {"password123"}, "password_confirm": {"password123"},
	})
	expectStatus(t, resp, http.StatusConflict)
	expectContains(t, body, `id="field-email-error"`, "déjà utilisée")
}

func TestRegisterLogsTheUserIn(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/register")
	resp, _ := c.post("/register", url.Values{
		"csrf_token": {token}, "username": {"Max"}, "email": {"new@test.fr"},
		"password": {"password123"}, "password_confirm": {"password123"},
	})
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/spaces" {
		t.Errorf("Location = %q, want /spaces", got)
	}

	resp, body := c.get("/spaces")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "Bienvenue Max", `role="status"`)
}

func TestRejectedTokenIsRemoved(t *testing.T) {
	c := newTestClient(t)
	u, _ := url.Parse(c.base)
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: "gdt_session", Value: "expired-token", Path: "/"}})

	resp, _ := c.get("/spaces")
	expectStatus(t, resp, http.StatusSeeOther)
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "gdt_session" {
			t.Errorf("the rejected token must be removed from the browser")
		}
	}
}

func TestLoginRedirectIgnoresExternalNext(t *testing.T) {
	c := newTestClient(t)
	token := c.csrfToken("/login")
	resp, _ := c.post("/login", url.Values{
		"csrf_token": {token}, "next": {"//evil.example/steal"},
		"email": {"ok@test.fr"}, "password": {"password123"},
	})
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/spaces" {
		t.Errorf("Location = %q, want /spaces (open redirect)", got)
	}
}
