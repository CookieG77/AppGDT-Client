// Login, registration and logout pages

package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

// homeAfterLogin is the page shown once logged in.
const homeAfterLogin = "/spaces"

type AuthHandler struct {
	api      *apiclient.Client
	renderer *view.Renderer
	sessions *session.Manager
}

func NewAuthHandler(api *apiclient.Client, renderer *view.Renderer, sessions *session.Manager) *AuthHandler {
	return &AuthHandler{api: api, renderer: renderer, sessions: sessions}
}

// --- Login ---------------------------------------------------------------------

var loginFields = []string{"email", "password"}

type loginPage struct {
	Form *Form
	// Next is the page to go back to after logging in
	Next string
}

func (h *AuthHandler) LoginForm(w http.ResponseWriter, r *http.Request) {
	h.renderLogin(w, r, http.StatusOK, loginPage{Form: emptyForm(), Next: safeNext(r.URL.Query().Get("next"))})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	form := newForm(r, loginFields, "password")
	page := loginPage{Form: form, Next: safeNext(r.PostFormValue("next"))}

	form.Required(map[string]string{
		"email":    "Saisissez votre adresse email.",
		"password": "Saisissez votre mot de passe.",
	}, loginFields...)
	if form.HasErrors() {
		h.renderLogin(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	token, err := h.api.Login(r.Context(), apiclient.LoginInput{
		Email:    form.Get("email"),
		Password: form.Get("password"),
	})
	if err != nil {
		status := http.StatusUnauthorized
		if apiErr, ok := apiclient.AsAPIError(err); ok && apiErr.Status == http.StatusUnauthorized {
			// Same message whatever was wrong, so that nobody can find out
			// which email addresses have an account
			form.Message = "Email ou mot de passe incorrect."
		} else {
			status = handleAPIError(r, form, err, loginFields)
		}
		h.renderLogin(w, r, status, page)
		return
	}

	h.sessions.SetToken(w, *token)
	http.Redirect(w, r, page.Next, http.StatusSeeOther)
}

func (h *AuthHandler) renderLogin(w http.ResponseWriter, r *http.Request, status int, page loginPage) {
	h.renderer.Render(w, r, status, "login", view.Page{
		Title:       formTitle("Connexion", page.Form),
		Description: "Connectez-vous à GDT pour retrouver vos espaces et vos notes.",
		Data:        page,
	})
}

// --- Registration --------------------------------------------------------------

var registerFields = []string{"username", "email", "password", "password_confirm"}

// Limits of the API, checked here too so that the user gets every error at once
const (
	usernameMaxLength = 50
	passwordMinLength = 8
	passwordMaxBytes  = 72
)

type registerPage struct {
	Form *Form
}

func (h *AuthHandler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	h.renderRegister(w, r, http.StatusOK, registerPage{Form: emptyForm()})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	form := newForm(r, registerFields, "password", "password_confirm")
	page := registerPage{Form: form}

	validateRegistration(form)
	if form.HasErrors() {
		h.renderRegister(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	ctx := r.Context()
	user, err := h.api.Register(ctx, apiclient.RegisterInput{
		Email:    form.Get("email"),
		Username: form.Get("username"),
		Password: form.Get("password"),
	})
	if err != nil {
		var status int
		if apiErr, ok := apiclient.AsAPIError(err); ok && apiErr.Status == http.StatusConflict {
			form.AddError("email", apiErr.Message)
			status = http.StatusConflict
		} else {
			status = handleAPIError(r, form, err, registerFields)
		}
		h.renderRegister(w, r, status, page)
		return
	}

	// The account exists: log the user in right away
	if !h.login(ctx, w, form.Get("email"), form.Get("password")) {
		h.sessions.SetFlash(w, "success", "Votre compte a été créé. Vous pouvez maintenant vous connecter.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.sessions.SetFlash(w, "success", "Bienvenue "+user.Username+" ! Votre compte a été créé.")
	http.Redirect(w, r, homeAfterLogin, http.StatusSeeOther)
}

// validateRegistration checks what can be checked before calling the API.
func validateRegistration(form *Form) {
	form.Required(map[string]string{
		"username":         "Saisissez un nom ou un pseudo.",
		"email":            "Saisissez votre adresse email.",
		"password":         "Choisissez un mot de passe.",
		"password_confirm": "Confirmez votre mot de passe.",
	}, registerFields...)

	if n := utf8.RuneCountInString(form.Get("username")); n > usernameMaxLength {
		form.AddError("username", "Le nom ne doit pas dépasser 50 caractères.")
	}
	if email := form.Get("email"); email != "" && !strings.Contains(email, "@") {
		form.AddError("email", "Saisissez une adresse email valide, par exemple nom@exemple.fr.")
	}
	if password := form.Get("password"); password != "" {
		switch {
		case utf8.RuneCountInString(password) < passwordMinLength:
			form.AddError("password", "Le mot de passe doit contenir au moins 8 caractères.")
		case len(password) > passwordMaxBytes:
			form.AddError("password", "Le mot de passe est trop long (72 octets maximum, les caractères accentués en comptent 2).")
		}
	}
	if confirm := form.Get("password_confirm"); confirm != "" && confirm != form.Get("password") {
		form.AddError("password_confirm", "Les deux mots de passe ne correspondent pas.")
	}
}

func (h *AuthHandler) renderRegister(w http.ResponseWriter, r *http.Request, status int, page registerPage) {
	h.renderer.Render(w, r, status, "register", view.Page{
		Title:       formTitle("Créer un compte", page.Form),
		Description: "Créez votre compte GDT pour organiser vos notes par espaces.",
		Data:        page,
	})
}

// login asks the API for a token and stores it. It reports whether it worked.
func (h *AuthHandler) login(ctx context.Context, w http.ResponseWriter, email, password string) bool {
	token, err := h.api.Login(ctx, apiclient.LoginInput{Email: email, Password: password})
	if err != nil {
		return false
	}
	h.sessions.SetToken(w, *token)
	return true
}

// --- Logout --------------------------------------------------------------------

// Logout removes the token from the browser.
// The JWT itself stays valid until it expires: the API cannot revoke it
// (see the limits described in the documentation).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.sessions.ClearToken(w)
	h.sessions.SetFlash(w, "info", "Vous êtes déconnecté. À bientôt !")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- Helpers -------------------------------------------------------------------

// formTitle prefixes the page title when the form has errors, so that screen
// reader users know it right away (RGAA recommendation).
func formTitle(title string, form *Form) string {
	if form.HasErrors() {
		return "Erreur : " + title
	}
	return title
}

// safeNext only accepts a path of this site as redirection target after
// login. Anything else ("https://evil.example", "//evil.example"…) would
// allow an open redirect and is replaced by the default page.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.Contains(next, "\\") {
		return homeAfterLogin
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return homeAfterLogin
	}
	return next
}
