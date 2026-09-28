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

	"github.com/CookieG77/AppGDT-Client/internal/config"
	"github.com/CookieG77/AppGDT-Client/internal/handler"
	"github.com/CookieG77/AppGDT-Client/internal/server"
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

	// Parsing every template once: an invalid template stops the startup
	renderer, err := view.NewRenderer(web.FS)
	if err != nil {
		return err
	}

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return fmt.Errorf("loading static files failed: %w", err)
	}

	// Creating handlers and server
	handlers := server.Handlers{
		Page: handler.NewPageHandler(renderer),
	}

	addr := cfg.Address + ":" + strconv.Itoa(cfg.Port)
	srv := server.New(addr, handlers, staticFS)

	// Starting the server in a goroutine to prevent a freeze of the exit signal waiter
	servErr := make(chan error, 1)
	go func() {
		logger.Info("client starting", "addr", addr, "api", cfg.API.BaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
