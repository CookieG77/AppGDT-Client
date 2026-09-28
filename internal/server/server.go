// Handles the routing of all client pages

package server

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/middleware"
)

type Handlers struct {
	Page *handler.PageHandler
}

// Options holds the transport settings of the server.
type Options struct {
	// TLS enables HTTPS when not nil (see LoadTLSConfig)
	TLS *tls.Config
	// HSTS sends the Strict-Transport-Security header (HTTPS only)
	HSTS bool
}

func New(addr string, h Handlers, staticFS fs.FS, opts Options) *http.Server {
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
