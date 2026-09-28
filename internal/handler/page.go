// Handlers for the pages that do not depend on the API

package handler

import (
	"net/http"

	"github.com/CookieG77/AppGDT-Client/internal/view"
)

type PageHandler struct {
	renderer *view.Renderer
}

func NewPageHandler(renderer *view.Renderer) *PageHandler {
	return &PageHandler{renderer: renderer}
}

// Home renders the landing page.
func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	h.renderer.Render(w, http.StatusOK, "home", view.Page{})
}

// NotFound renders the 404 page for any URL matching no route.
func (h *PageHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	h.renderer.RenderError(w, http.StatusNotFound, "La page demandée n'existe pas.")
}

// InternalError renders the 500 page, used after a recovered panic.
func (h *PageHandler) InternalError(w http.ResponseWriter, r *http.Request) {
	h.renderer.RenderError(w, http.StatusInternalServerError, "Une erreur interne est survenue.")
}
