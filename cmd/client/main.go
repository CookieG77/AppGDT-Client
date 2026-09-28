package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/config"
	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/server"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
	"github.com/CookieG77/AppGDT-Client/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("Client encountered an error and was stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Loading the client config
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Client used by every page to call the API. An unreachable API does not
	// stop the startup: pages will show an error until it comes back.
	api := apiclient.New(cfg.API.BaseURL, cfg.API.Timeout)
	checkAPI(ctx, logger, api, cfg.API.BaseURL)

	// Parsing every template once: an invalid template stops the startup
	renderer, err := view.NewRenderer(web.FS)
	if err != nil {
		return err
	}

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return fmt.Errorf("loading static files failed: %w", err)
	}

	// Session cookies are Secure (HTTPS only) when TLS is enabled
	sessions := session.NewManager(cfg.TLS.Enabled())

	// Creating handlers and server
	handlers := server.Handlers{
		Page:  handler.NewPageHandler(renderer),
		Auth:  handler.NewAuthHandler(api, renderer, sessions),
		Space: handler.NewSpaceHandler(api, renderer, sessions),
		Note:  handler.NewNoteHandler(api, renderer, sessions),

		Account: handler.NewAccountHandler(api, renderer, sessions),
	}

	// HTTPS between the browser and the client, if a certificate is configured
	opts := server.Options{HSTS: cfg.TLS.HSTS}
	scheme := "http"
	if cfg.TLS.Enabled() {
		if opts.TLS, err = server.LoadTLSConfig(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
			return err
		}
		scheme = "https"
	} else if cfg.TLS.HSTS {
		logger.Warn("TLS_HSTS is ignored without TLS_CERT_FILE and TLS_KEY_FILE")
	}

	addr := cfg.Address + ":" + strconv.Itoa(cfg.Port)
	srv := server.New(addr, handlers, server.Sessions{Manager: sessions, API: api}, staticFS, opts)

	// Starting the server in a goroutine to prevent a freeze of the exit signal waiter
	servErr := make(chan error, 1)
	go func() {
		logger.Info("client starting", "url", scheme+"://"+addr, "api", cfg.API.BaseURL)
		if err := listen(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			servErr <- err
		}
	}()

	// Waiting for server error or exit signal
	select {
	case err := <-servErr:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		logger.Info("client stopping")
	}

	// Clean stop: gives time for request to end before stopping the app
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	logger.Info("client stopped")
	return nil
}

// listen serves HTTPS when the server has a TLS config, HTTP otherwise.
func listen(srv *http.Server) error {
	if srv.TLSConfig != nil {
		// The certificate is already loaded in TLSConfig
		return srv.ListenAndServeTLS("", "")
	}
	return srv.ListenAndServe()
}

// checkAPI logs whether the API answers at startup.
func checkAPI(ctx context.Context, logger *slog.Logger, api *apiclient.Client, baseURL string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := api.Health(ctx); err != nil {
		logger.Warn("api is not reachable yet", "api", baseURL, "error", err)
		return
	}
	logger.Info("api is reachable", "api", baseURL)
}
