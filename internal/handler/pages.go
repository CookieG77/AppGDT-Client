// Helpers shared by the pages that show data of the API

package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

// confirmDeletePage is the data of the generic confirmation page.
type confirmDeletePage struct {
	Heading     string
	Message     string
	Action      string
	ButtonLabel string
	CancelURL   string
}

func spaceURL(id int64) string {
	return "/spaces/" + strconv.FormatInt(id, 10)
}

// token returns the JWT of the authenticated user (pages behind RequireAuth).
func token(r *http.Request) string {
	if current := session.CurrentFrom(r.Context()); current != nil {
		return current.Token
	}
	return ""
}

// pathID reads a positive numeric ID from the URL. An invalid ID gives a 404:
// for the user, "/spaces/abc" is simply a page that does not exist.
func pathID(w http.ResponseWriter, r *http.Request, renderer *view.Renderer, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		renderer.RenderError(w, r, http.StatusNotFound, "La page demandée n'existe pas.")
		return 0, false
	}
	return id, true
}

// handleSessionExpired sends the user back to the login page when the API
// refuses the token. It reports whether it answered the request.
func handleSessionExpired(w http.ResponseWriter, r *http.Request, sessions *session.Manager, err error) bool {
	if !apiclient.IsUnauthorized(err) {
		return false
	}
	sessions.ClearToken(w)
	sessions.SetFlash(w, "info", "Votre session a expiré. Reconnectez-vous.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return true
}

// apiPageError answers a failed API call on a page that shows data.
func apiPageError(w http.ResponseWriter, r *http.Request, renderer *view.Renderer, sessions *session.Manager, err error) {
	if handleSessionExpired(w, r, sessions, err) {
		return
	}
	switch {
	case apiclient.IsNotFound(err):
		// The API answers 404 for missing resources and for resources of
		// other users alike, so nothing reveals that they exist
		renderer.RenderError(w, r, http.StatusNotFound, "Cette page n'existe pas ou ne vous appartient pas.")
	case errors.Is(err, apiclient.ErrUnavailable):
		renderer.RenderError(w, r, http.StatusServiceUnavailable, "Le service est momentanément indisponible. Réessayez dans quelques instants.")
	default:
		slog.ErrorContext(r.Context(), "unexpected API error", "error", err, "path", r.URL.Path)
		renderer.RenderError(w, r, http.StatusInternalServerError, "Une erreur interne est survenue.")
	}
}
