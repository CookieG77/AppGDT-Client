// Package tests holds the end-to-end tests of the client: they start the
// whole client (routes, middlewares, handlers, templates) on top of a fake
// API kept in memory, and browse it like a user would, with cookies and forms.
// The unit tests stay next to the code they test (*_test.go files), as Go
// requires to reach unexported functions.
package tests

import (
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/server"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
	"github.com/CookieG77/AppGDT-Client/web"
)

// TestMain silences the logs of the client during the tests.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

// fakeAPI imitates the routes of AppGDT-Server used by the pages.
// Spaces are kept in memory; the token "valid-token" belongs to user 1.
type fakeAPI struct {
	calls atomic.Int32

	mu      sync.Mutex
	nextID  int64
	spaces  map[int64]*apiclient.Space
	notes   map[int64]*apiclient.Note
	deleted bool // the account of user 1 was deleted
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{nextID: 1, spaces: map[int64]*apiclient.Space{}, notes: map[int64]*apiclient.Note{}}
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()

	writeJSON := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	apiError := func(status int, code, msg string) {
		writeJSON(status, map[string]string{"code": code, "message": msg})
	}
	notFound := func() { apiError(http.StatusNotFound, "NOT_FOUND", "Ressource introuvable.") }

	public := r.URL.Path == "/auth/login" || r.URL.Path == "/auth/register"
	if !public && r.Header.Get("Authorization") != "Bearer valid-token" {
		apiError(http.StatusUnauthorized, "UNAUTHORIZED", "Authentification requise.")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
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
	})
	mux.HandleFunc("POST /auth/register", func(w http.ResponseWriter, r *http.Request) {
		var in apiclient.RegisterInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Email == "taken@test.fr" {
			apiError(http.StatusConflict, "EMAIL_ALREADY_USED", "Cette adresse email est déjà utilisée.")
			return
		}
		writeJSON(http.StatusCreated, apiclient.User{ID: 2, Email: in.Email, Username: in.Username})
	})
	mux.HandleFunc("GET /users/me", func(w http.ResponseWriter, r *http.Request) {
		if f.deleted {
			apiError(http.StatusUnauthorized, "UNAUTHORIZED", "Authentification requise.")
			return
		}
		writeJSON(http.StatusOK, apiclient.User{ID: 1, Email: "ok@test.fr", Username: "Maxime"})
	})
	mux.HandleFunc("GET /users/me/export", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="gdt-export-2026-09-28.json"`)
		writeJSON(http.StatusOK, map[string]any{"user": map[string]string{"email": "ok@test.fr"}, "spaces": []any{}})
	})
	mux.HandleFunc("DELETE /users/me", func(w http.ResponseWriter, r *http.Request) {
		var in apiclient.DeleteAccountInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Password != "password123" {
			apiError(http.StatusForbidden, "INVALID_PASSWORD", "Mot de passe incorrect.")
			return
		}
		f.deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /spaces", func(w http.ResponseWriter, r *http.Request) {
		list := []apiclient.Space{}
		for _, sp := range f.spaces {
			list = append(list, *sp)
		}
		writeJSON(http.StatusOK, list)
	})
	mux.HandleFunc("POST /spaces", func(w http.ResponseWriter, r *http.Request) {
		var in apiclient.SpaceInput
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Name == "invalide" {
			writeJSON(http.StatusBadRequest, map[string]any{"code": "VALIDATION_ERROR", "message": "Les données envoyées sont invalides.",
				"details": []map[string]string{{"field": "name", "message": "Nom refusé par l'API."}}})
			return
		}
		sp := &apiclient.Space{ID: f.nextID, Name: in.Name, Description: in.Description, UpdatedAt: time.Now()}
		f.spaces[sp.ID] = sp
		f.nextID++
		writeJSON(http.StatusCreated, sp)
	})
	withSpace := func(handle func(sp *apiclient.Space)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
			sp, ok := f.spaces[id]
			if !ok {
				notFound()
				return
			}
			handle(sp)
		}
	}
	mux.HandleFunc("GET /spaces/{id}", withSpace(func(sp *apiclient.Space) { writeJSON(http.StatusOK, sp) }))
	mux.HandleFunc("PUT /spaces/{id}", func(w http.ResponseWriter, r *http.Request) {
		withSpace(func(sp *apiclient.Space) {
			var in apiclient.SpaceInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			sp.Name, sp.Description = in.Name, in.Description
			writeJSON(http.StatusOK, sp)
		})(w, r)
	})
	mux.HandleFunc("DELETE /spaces/{id}", withSpace(func(sp *apiclient.Space) {
		delete(f.spaces, sp.ID)
		for id, n := range f.notes {
			if n.SpaceID == sp.ID {
				delete(f.notes, id)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("GET /spaces/{id}/notes", withSpace(func(sp *apiclient.Space) {
		list := []apiclient.Note{}
		for _, n := range f.notes {
			if n.SpaceID == sp.ID {
				list = append(list, *n)
			}
		}
		writeJSON(http.StatusOK, list)
	}))
	mux.HandleFunc("POST /spaces/{id}/notes", func(w http.ResponseWriter, r *http.Request) {
		withSpace(func(sp *apiclient.Space) {
			var in apiclient.NoteInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			now := time.Now()
			n := &apiclient.Note{ID: f.nextID, SpaceID: sp.ID, Title: in.Title, Content: in.Content, Status: in.Status, CreatedAt: now, UpdatedAt: now}
			f.notes[n.ID] = n
			f.nextID++
			writeJSON(http.StatusCreated, n)
		})(w, r)
	})
	withNote := func(handle func(n *apiclient.Note)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
			n, ok := f.notes[id]
			if !ok {
				notFound()
				return
			}
			handle(n)
		}
	}
	mux.HandleFunc("GET /notes/{id}", withNote(func(n *apiclient.Note) { writeJSON(http.StatusOK, n) }))
	mux.HandleFunc("PUT /notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		withNote(func(n *apiclient.Note) {
			var in apiclient.NoteInput
			_ = json.NewDecoder(r.Body).Decode(&in)
			n.Title, n.Content, n.Status, n.UpdatedAt = in.Title, in.Content, in.Status, time.Now()
			writeJSON(http.StatusOK, n)
		})(w, r)
	})
	mux.HandleFunc("DELETE /notes/{id}", withNote(func(n *apiclient.Note) {
		delete(f.notes, n.ID)
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		apiError(http.StatusNotFound, "ROUTE_NOT_FOUND", "Route inconnue.")
	})
	mux.ServeHTTP(w, r)
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

	api := newFakeAPI()
	apiSrv := httptest.NewServer(api)
	t.Cleanup(apiSrv.Close)

	renderer, err := view.NewRenderer(web.FS)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	staticFS, _ := fs.Sub(web.FS, "static")
	apiClient := apiclient.New(apiSrv.URL, 2*time.Second)
	sessions := session.NewManager(false)

	srv := server.New("", server.Handlers{
		Page:  handler.NewPageHandler(renderer),
		Auth:  handler.NewAuthHandler(apiClient, renderer, sessions),
		Space: handler.NewSpaceHandler(apiClient, renderer, sessions),
		Note:  handler.NewNoteHandler(apiClient, renderer, sessions),

		Account: handler.NewAccountHandler(apiClient, renderer, sessions),
	}, server.Sessions{Manager: sessions, API: apiClient}, staticFS, server.Options{})

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
	expectContains(t, body, "Mes espaces", "Maxime", `action="/logout"`)
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

// --- Spaces ----------------------------------------------------------------------

func TestSpaceLifecycle(t *testing.T) {
	c := newTestClient(t)
	c.login()

	resp, body := c.get("/spaces")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "Votre carnet est vide")

	token := c.csrfToken("/spaces/new")

	// Creation
	resp, _ = c.post("/spaces", url.Values{"csrf_token": {token}, "name": {"  Devoirs  "}, "description": {"École"}})
	expectStatus(t, resp, http.StatusSeeOther)
	location := resp.Header.Get("Location")
	if location != "/spaces/1" {
		t.Fatalf("Location = %q, want /spaces/1", location)
	}
	resp, body = c.get(location)
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "<h1>Devoirs</h1>", "a été créé", `class="breadcrumb" aria-label=`)

	// Listed, with a link to the space
	_, body = c.get("/spaces")
	expectContains(t, body, `href="/spaces/1"`, "École")

	// Modification: the form is pre-filled
	_, body = c.get("/spaces/1/edit")
	expectContains(t, body, `value="Devoirs"`, "École</textarea>")
	resp, _ = c.post("/spaces/1", url.Values{"csrf_token": {token}, "name": {"Cours"}, "description": {""}})
	expectStatus(t, resp, http.StatusSeeOther)
	_, body = c.get("/spaces/1")
	expectContains(t, body, "<h1>Cours</h1>", "a été modifié")

	// Deletion goes through a confirmation page
	resp, body = c.get("/spaces/1/delete")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "irréversible", `action="/spaces/1/delete"`)
	resp, _ = c.post("/spaces/1/delete", url.Values{"csrf_token": {token}})
	expectStatus(t, resp, http.StatusSeeOther)
	resp, _ = c.get("/spaces/1")
	expectStatus(t, resp, http.StatusNotFound)
}

func TestSpaceFormErrors(t *testing.T) {
	c := newTestClient(t)
	c.login()
	token := c.csrfToken("/spaces/new")

	// Checked by the client: no API call for an empty name
	before := c.api.calls.Load()
	resp, body := c.post("/spaces", url.Values{"csrf_token": {token}, "name": {"   "}, "description": {"Gardée"}})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body, "Donnez un nom à l&#39;espace.", "Gardée</textarea>", "<title>Erreur : Nouvel espace · GDT</title>")
	// Only the session check (GET /users/me) reached the API
	if calls := c.api.calls.Load() - before; calls != 1 {
		t.Errorf("expected only the session check, got %d API calls", calls)
	}

	// Refused by the API: its message is shown under the field
	resp, body = c.post("/spaces", url.Values{"csrf_token": {token}, "name": {"invalide"}})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body, "Nom refusé par l&#39;API.", `id="field-name-error"`)
}

func TestUnknownOrInvalidSpaceGives404(t *testing.T) {
	c := newTestClient(t)
	c.login()
	for _, path := range []string{"/spaces/99", "/spaces/abc", "/spaces/0/edit", "/spaces/-1/delete"} {
		resp, _ := c.get(path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestSpacePagesRequireLogin(t *testing.T) {
	c := newTestClient(t)
	for _, path := range []string{"/spaces/new", "/spaces/1", "/spaces/1/edit", "/spaces/1/delete"} {
		resp, _ := c.get(path)
		if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/login?next=") {
			t.Errorf("GET %s: expected a redirection to the login page, got %d %s", path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

// --- Notes -----------------------------------------------------------------------

// createSpace creates a space through the pages and returns its URL.
func (c *testClient) createSpace(name string) string {
	c.t.Helper()
	token := c.csrfToken("/spaces/new")
	resp, _ := c.post("/spaces", url.Values{"csrf_token": {token}, "name": {name}})
	if resp.StatusCode != http.StatusSeeOther {
		c.t.Fatalf("space creation failed with status %d", resp.StatusCode)
	}
	return resp.Header.Get("Location")
}

func TestNoteLifecycle(t *testing.T) {
	c := newTestClient(t)
	c.login()
	spacePath := c.createSpace("Devoirs")

	resp, body := c.get(spacePath)
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "ne contient pas encore de note", `href="`+spacePath+`/notes/new"`)

	// Creation from the space, "todo" checked by default
	_, body = c.get(spacePath + "/notes/new")
	expectContains(t, body, `value="todo" id="field-status" checked`, "Devoirs&nbsp;»")
	token := c.csrfToken(spacePath + "/notes/new")
	resp, _ = c.post(spacePath+"/notes", url.Values{
		"csrf_token": {token}, "title": {"Exercices p.52"},
		"content": {"Ligne 1\nLigne 2 <b>pas du HTML</b>"}, "status": {"in_progress"},
	})
	expectStatus(t, resp, http.StatusSeeOther)
	notePath := resp.Header.Get("Location")
	if !strings.HasPrefix(notePath, "/notes/") {
		t.Fatalf("Location = %q, want /notes/{id}", notePath)
	}

	// Reading mode: content escaped, status as text, breadcrumb up to the space
	resp, body = c.get(notePath)
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "Exercices p.52", "En cours", "&lt;b&gt;pas du HTML&lt;/b&gt;", `href="`+spacePath+`"`)

	// Listed in its space, with a filter link per status and its count
	_, body = c.get(spacePath)
	expectContains(t, body, `href="`+notePath+`"`, `aria-label="Filtrer les notes par état"`, `En cours</span>&nbsp;: 1`)

	// Filtered on its status it is listed, on another status it is not
	_, body = c.get(spacePath + "?status=in_progress")
	expectContains(t, body, `href="`+notePath+`"`, `?status=in_progress" aria-current="true"`)
	_, body = c.get(spacePath + "?status=done")
	expectContains(t, body, "Aucune note", "Terminé")
	if strings.Contains(body, `href="`+notePath+`"`) {
		t.Error("the filter must hide the notes of other statuses")
	}
	// An unknown status shows every note
	_, body = c.get(spacePath + "?status=urgent")
	expectContains(t, body, `href="`+notePath+`"`)

	// Editing mode: pre-filled, then saved
	_, body = c.get(notePath + "/edit")
	expectContains(t, body, `value="Exercices p.52"`, `value="in_progress" id="field-status-in_progress" checked`)
	resp, _ = c.post(notePath, url.Values{"csrf_token": {token}, "title": {"Exercices p.53"}, "content": {""}, "status": {"done"}})
	expectStatus(t, resp, http.StatusSeeOther)
	_, body = c.get(notePath)
	expectContains(t, body, "Exercices p.53", "Terminé", "pas encore de contenu")

	// Deletion, then back to the space
	resp, body = c.get(notePath + "/delete")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "irréversible")
	resp, _ = c.post(notePath+"/delete", url.Values{"csrf_token": {token}})
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != spacePath {
		t.Errorf("Location = %q, want %q", got, spacePath)
	}
	resp, _ = c.get(notePath)
	expectStatus(t, resp, http.StatusNotFound)
}

func TestNoteFormErrors(t *testing.T) {
	c := newTestClient(t)
	c.login()
	spacePath := c.createSpace("Jobs")
	token := c.csrfToken(spacePath + "/notes/new")

	resp, body := c.post(spacePath+"/notes", url.Values{
		"csrf_token": {token}, "title": {""}, "content": {"Gardé"}, "status": {"urgent"},
	})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body,
		"Le formulaire contient 2 erreurs",
		`href="#field-title"`, `href="#field-status"`,
		`aria-describedby="field-status-error"`,
		"Gardé</textarea>",
	)
}

func TestNoteOfUnknownSpace(t *testing.T) {
	c := newTestClient(t)
	c.login()
	for _, path := range []string{"/spaces/42/notes/new", "/notes/42", "/notes/42/edit", "/notes/x/delete"} {
		resp, _ := c.get(path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, resp.StatusCode)
		}
	}
}

// --- Markdown ----------------------------------------------------------------------

// The Markdown rendering happens in the browser: the pages only have to
// send the raw content and load the scripts, which must be served locally
// (the CSP forbids external scripts).
func TestMarkdownScriptsAreServedLocally(t *testing.T) {
	c := newTestClient(t)
	c.login()
	spacePath := c.createSpace("Notes")
	token := c.csrfToken(spacePath + "/notes/new")
	resp, _ := c.post(spacePath+"/notes", url.Values{
		"csrf_token": {token}, "title": {"Markdown"}, "status": {"todo"},
		"content": {"# Titre\n\n**gras** <script>alert(1)</script>"},
	})
	notePath := resp.Header.Get("Location")

	_, body := c.get(notePath)
	expectContains(t, body,
		`data-markdown data-first-heading="2"`,
		"# Titre",                               // raw Markdown, readable without JavaScript…
		"&lt;script&gt;alert(1)&lt;/script&gt;", // …and escaped by html/template
		`<script src="/static/vendor/marked.umd.js" defer></script>`,
		`<script src="/static/vendor/purify.min.js" defer></script>`,
		`<script src="/static/js/markdown.js" defer></script>`,
	)

	_, body = c.get(notePath + "/edit")
	expectContains(t, body, "data-markdown-editor", `<details class="markdown-help">`)

	for _, path := range []string{"/static/vendor/marked.umd.js", "/static/vendor/purify.min.js", "/static/js/markdown.js"} {
		resp, _ := c.get(path)
		expectStatus(t, resp, http.StatusOK)
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
			t.Errorf("%s: Content-Type = %q", path, ct)
		}
	}
}

// --- Account (GDPR rights) -----------------------------------------------------------

func TestAccountExport(t *testing.T) {
	c := newTestClient(t)
	c.login()

	resp, body := c.get("/account")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "ok@test.fr", `href="/account/export"`, `href="/account/delete"`, `aria-current="page"`)

	resp, body = c.get("/account/export")
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename=gdt-export-2026-09-28.json` {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	expectContains(t, body, `"email":"ok@test.fr"`)
}

func TestAccountDeletion(t *testing.T) {
	c := newTestClient(t)
	c.login()
	token := c.csrfToken("/account/delete")

	// The explicit confirmation is required
	resp, body := c.post("/account/delete", url.Values{"csrf_token": {token}, "password": {"password123"}})
	expectStatus(t, resp, http.StatusUnprocessableEntity)
	expectContains(t, body, "Cochez la case", `aria-describedby="field-confirm-error"`)

	// A wrong password keeps the account
	resp, body = c.post("/account/delete", url.Values{"csrf_token": {token}, "password": {"mauvais"}, "confirm": {"yes"}})
	expectStatus(t, resp, http.StatusForbidden)
	expectContains(t, body, "Mot de passe incorrect.", `id="field-password-error"`)
	if c.api.deleted {
		t.Fatal("the account must not be deleted with a wrong password")
	}

	// Right password: account deleted, session removed, back to the home page
	resp, _ = c.post("/account/delete", url.Values{"csrf_token": {token}, "password": {"password123"}, "confirm": {"yes"}})
	expectStatus(t, resp, http.StatusSeeOther)
	if got := resp.Header.Get("Location"); got != "/" {
		t.Errorf("Location = %q, want /", got)
	}
	_, body = c.get("/")
	expectContains(t, body, "définitivement supprimés", `href="/login"`)
	resp, _ = c.get("/account")
	expectStatus(t, resp, http.StatusSeeOther)
}

// --- Privacy page ----------------------------------------------------------------------

func TestPrivacyPage(t *testing.T) {
	c := newTestClient(t)

	// Public, and linked from every page and from the registration form
	resp, body := c.get("/privacy")
	expectStatus(t, resp, http.StatusOK)
	expectContains(t, body, "<h1 id=\"privacy-title\">Confidentialité</h1>", "gdt_session", "gdt_csrf", "gdt_flash", "https://www.cnil.fr/fr/plaintes")
	_, body = c.get("/")
	expectContains(t, body, `href="/privacy"`)
	_, body = c.get("/register")
	expectContains(t, body, `href="/privacy"`)

	// Logged in: the footer also links to the account page
	c.login()
	_, body = c.get("/privacy")
	expectContains(t, body, `<a href="/privacy" aria-current="page">`, `href="/account"`)
}

// --- Task lists -------------------------------------------------------------------------

func TestToggleTask(t *testing.T) {
	c := newTestClient(t)
	c.login()
	spacePath := c.createSpace("Tâches")
	token := c.csrfToken(spacePath + "/notes/new")
	resp, _ := c.post(spacePath+"/notes", url.Values{
		"csrf_token": {token}, "title": {"Courses"}, "status": {"todo"},
		"content": {"- [ ] pain\n- [x] lait"},
	})
	notePath := resp.Header.Get("Location")

	_, body := c.get(notePath)
	expectContains(t, body, `data-tasks-url="`+notePath+`/tasks"`, `data-csrf="`+token+`"`)

	// Tick the first task, untick the second one
	resp, _ = c.post(notePath+"/tasks", url.Values{"csrf_token": {token}, "index": {"0"}, "done": {"true"}})
	expectStatus(t, resp, http.StatusNoContent)
	resp, _ = c.post(notePath+"/tasks", url.Values{"csrf_token": {token}, "index": {"1"}, "done": {"false"}})
	expectStatus(t, resp, http.StatusNoContent)
	_, body = c.get(notePath + "/edit")
	expectContains(t, body, "- [x] pain\n- [ ] lait</textarea>")

	// Unknown task, invalid index, missing CSRF token
	resp, _ = c.post(notePath+"/tasks", url.Values{"csrf_token": {token}, "index": {"5"}, "done": {"true"}})
	expectStatus(t, resp, http.StatusBadRequest)
	resp, _ = c.post(notePath+"/tasks", url.Values{"csrf_token": {token}, "index": {"-1"}, "done": {"true"}})
	expectStatus(t, resp, http.StatusBadRequest)
	resp, _ = c.post(notePath+"/tasks", url.Values{"index": {"0"}, "done": {"true"}})
	expectStatus(t, resp, http.StatusForbidden)
}
