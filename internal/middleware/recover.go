package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches a panic in the next handlers, logs it with its stack trace
// and answers with the given error handler instead of dropping the connection.
func Recover(onPanic http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// http.ErrAbortHandler is used on purpose to abort a response
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.Error("panic recovered",
					"error", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				onPanic.ServeHTTP(w, r)
			}()

			next.ServeHTTP(w, r)
		})
	}
}
