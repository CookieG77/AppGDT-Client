package middleware

import "net/http"

// SecureHeaders adds security headers to every response:
//   - Content-Security-Policy: only resources from the client itself are allowed
//     (no inline script, no external resource)
//   - X-Content-Type-Options: the browser must not guess the content type
//   - X-Frame-Options / frame-ancestors: pages cannot be embedded (clickjacking)
//   - Referrer-Policy: URLs are not leaked to other sites
//   - Strict-Transport-Security (only if hsts is true, which requires HTTPS):
//     the browser must use HTTPS for this host during one year
func SecureHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return secureHeaders(next, hsts)
	}
}

func secureHeaders(next http.Handler, hsts bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if hsts {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
