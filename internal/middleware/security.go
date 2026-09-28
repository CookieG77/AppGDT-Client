package middleware

import "net/http"

// SecureHeaders adds security headers to every response:
//   - Content-Security-Policy: only resources from the client itself are allowed
//     (no inline script, no external resource)
//   - X-Content-Type-Options: the browser must not guess the content type
//   - X-Frame-Options / frame-ancestors: pages cannot be embedded (clickjacking)
//   - Referrer-Policy: URLs are not leaked to other sites
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
