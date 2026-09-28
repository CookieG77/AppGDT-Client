package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/CookieG77/AppGDT-Client/internal/session"
)

// maxFormSize limits the size of a submitted form (a note holds at most
// 50 000 characters, that is 200 KB in the worst case).
const maxFormSize = 1 << 20

// CSRF protects every form against cross-site request forgery:
//   - each page gets a token (cookie + hidden field, see session.CSRFToken)
//   - a request that changes something (POST…) must send back that token
//   - requests that the browser flags as coming from another site
//     (Sec-Fetch-Site header) are refused before even reading the form
func CSRF(sm *session.Manager, onError ErrorRenderer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := sm.CSRFToken(w, r)
			r = r.WithContext(session.WithCSRFToken(r.Context(), token))

			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
				slog.WarnContext(r.Context(), "cross-site form submission refused", "path", r.URL.Path, "site", site)
				onError(w, r, http.StatusForbidden, "Cette action doit être faite depuis GDT.")
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxFormSize)
			if err := r.ParseForm(); err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					onError(w, r, http.StatusRequestEntityTooLarge, "Le formulaire envoyé est trop volumineux.")
					return
				}
				onError(w, r, http.StatusBadRequest, "Le formulaire envoyé est invalide.")
				return
			}

			if !sm.CheckCSRF(r) {
				slog.WarnContext(r.Context(), "invalid CSRF token", "path", r.URL.Path)
				onError(w, r, http.StatusForbidden, "Le formulaire a expiré. Revenez en arrière, rechargez la page puis réessayez.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
