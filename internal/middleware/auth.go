package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
)

// ErrorRenderer renders an error page (see view.Renderer.RenderError).
type ErrorRenderer func(w http.ResponseWriter, r *http.Request, status int, message string)

// LoadSession reads the cookies of the request:
//   - the flash message, shown once then deleted
//   - the JWT, checked against the API (GET /users/me). A token refused by
//     the API (expired, deleted account) is removed from the browser.
//
// Asking the API on each page keeps the API as the only judge of a token's
// validity, at the cost of one extra call per page.
func LoadSession(api *apiclient.Client, sm *session.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if flash := sm.PopFlash(w, r); flash != nil {
				ctx = session.WithFlash(ctx, flash)
			}

			if token := sm.Token(r); token != "" {
				user, err := api.Me(ctx, token)
				switch {
				case err == nil:
					ctx = session.WithCurrent(ctx, &session.Current{User: user, Token: token})
				case apiclient.IsUnauthorized(err):
					sm.ClearToken(w)
				case errors.Is(err, apiclient.ErrUnavailable):
					ctx = session.WithAPIUnavailable(ctx)
				default:
					slog.ErrorContext(ctx, "session check failed", "error", err)
					ctx = session.WithAPIUnavailable(ctx)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth lets only authenticated users through. Visitors are sent to
// the login page, which brings them back here once logged in.
func RequireAuth(onError ErrorRenderer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Private pages must not be kept by the browser or a proxy:
			// after logging out, "back" must not show them again
			w.Header().Set("Cache-Control", "no-store")

			if session.CurrentFrom(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}

			if session.APIUnavailable(r.Context()) {
				onError(w, r, http.StatusServiceUnavailable, "Le service est momentanément indisponible. Réessayez dans quelques instants.")
				return
			}

			target := "/login"
			if r.Method == http.MethodGet {
				target += "?" + url.Values{"next": {r.URL.RequestURI()}}.Encode()
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
		})
	}
}

// RequireGuest sends authenticated users away from the login and register pages.
func RequireGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session.CurrentFrom(r.Context()) != nil {
			http.Redirect(w, r, "/spaces", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
