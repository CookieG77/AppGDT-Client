// Handles the routing of all client pages

package server

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/middleware"
	"github.com/CookieG77/AppGDT-Client/internal/session"
)

type Handlers struct {
	Page  *handler.PageHandler
	Auth  *handler.AuthHandler
	Space *handler.SpaceHandler
}

// Sessions gives the middlewares what they need to read the session.
type Sessions struct {
	Manager *session.Manager
	API     *apiclient.Client
}

// Options holds the transport settings of the server.
type Options struct {
	// TLS enables HTTPS when not nil (see LoadTLSConfig)
	TLS *tls.Config
	// HSTS sends the Strict-Transport-Security header (HTTPS only)
	HSTS bool
}

func New(addr string, h Handlers, s Sessions, staticFS fs.FS, opts Options) *http.Server {
	requireAuth := middleware.RequireAuth(h.Page.Error)
	guestOnly := middleware.RequireGuest

	pages := http.NewServeMux()

	// Public pages
	pages.HandleFunc("GET /{$}", h.Page.Home)

	// Authentication (login and register are for visitors only)
	pages.Handle("GET /login", guestOnly(http.HandlerFunc(h.Auth.LoginForm)))
	pages.Handle("POST /login", guestOnly(http.HandlerFunc(h.Auth.Login)))
	pages.Handle("GET /register", guestOnly(http.HandlerFunc(h.Auth.RegisterForm)))
	pages.Handle("POST /register", guestOnly(http.HandlerFunc(h.Auth.Register)))
	pages.HandleFunc("POST /logout", h.Auth.Logout)

	// Protected pages
	pages.Handle("GET /spaces", requireAuth(http.HandlerFunc(h.Space.List)))

	// Any other URL gets the HTML 404 page
	pages.HandleFunc("/", h.Page.NotFound)

	// Every page reads the session and is protected against CSRF
	loadSession := middleware.LoadSession(s.API, s.Manager)
	csrf := middleware.CSRF(s.Manager, h.Page.Error)

	mux := http.NewServeMux()
	// Static files (CSS, fonts, images), embedded in the binary. They skip
	// the session middlewares: no API call is needed to serve them.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
	mux.Handle("/", loadSession(csrf(pages)))

	// Each request goes through: request logging, security headers,
	// panic recovery, then the router
	recoverer := middleware.Recover(http.HandlerFunc(h.Page.InternalError))
	secureHeaders := middleware.SecureHeaders(opts.TLS != nil && opts.HSTS)
	handler := middleware.LogRequests(secureHeaders(recoverer(mux)))

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         opts.TLS,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Leaves time for the API call(s) made while building a page
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// LoadTLSConfig loads the certificate and its private key, so that a missing
// or invalid file stops the startup with a clear error.
// TLS 1.2 is the oldest accepted version; Go chooses secure cipher suites.
func LoadTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("loading TLS certificate failed: %w", err)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}, nil
}
