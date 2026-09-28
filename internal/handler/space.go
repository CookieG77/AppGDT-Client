// Space pages

package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

type SpaceHandler struct {
	api      *apiclient.Client
	renderer *view.Renderer
	sessions *session.Manager
}

func NewSpaceHandler(api *apiclient.Client, renderer *view.Renderer, sessions *session.Manager) *SpaceHandler {
	return &SpaceHandler{api: api, renderer: renderer, sessions: sessions}
}

type spaceListPage struct {
	Spaces []apiclient.Space
}

// List shows the spaces of the user.
func (h *SpaceHandler) List(w http.ResponseWriter, r *http.Request) {
	current := session.CurrentFrom(r.Context())

	spaces, err := h.api.ListSpaces(r.Context(), current.Token)
	if err != nil {
		h.apiError(w, r, err)
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "spaces", view.Page{
		Title: "Mes espaces",
		Data:  spaceListPage{Spaces: spaces},
	})
}

// apiError answers a failed API call on a page that shows data.
func (h *SpaceHandler) apiError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case apiclient.IsUnauthorized(err):
		// The token expired between the session check and this call
		h.sessions.ClearToken(w)
		h.sessions.SetFlash(w, "info", "Votre session a expiré. Reconnectez-vous.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case apiclient.IsNotFound(err):
		h.renderer.RenderError(w, r, http.StatusNotFound, "Cette page n'existe pas ou ne vous appartient pas.")
	case errors.Is(err, apiclient.ErrUnavailable):
		h.renderer.RenderError(w, r, http.StatusServiceUnavailable, "Le service est momentanément indisponible. Réessayez dans quelques instants.")
	default:
		slog.ErrorContext(r.Context(), "unexpected API error", "error", err, "path", r.URL.Path)
		h.renderer.RenderError(w, r, http.StatusInternalServerError, "Une erreur interne est survenue.")
	}
}
