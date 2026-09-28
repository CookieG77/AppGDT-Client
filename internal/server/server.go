// Handles the routing of all client pages

package server

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/middleware"
)

type Handlers struct {
	Page *handler.PageHandler
}

func New(addr string, h Handlers, staticFS fs.FS) *http.Server {
	mux := http.NewServeMux()

	// Static files (CSS, images), embedded in the binary
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Public pages
	mux.HandleFunc("GET /{$}", h.Page.Home)

	// Any other URL gets the HTML 404 page
	mux.HandleFunc("/", h.Page.NotFound)

	// Each request goes through: request logging, security headers,
	// panic recovery, then the router
	recoverer := middleware.Recover(http.HandlerFunc(h.Page.InternalError))
	handler := middleware.LogRequests(middleware.SecureHeaders(recoverer(mux)))

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Leaves time for the API call(s) made while building a page
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
