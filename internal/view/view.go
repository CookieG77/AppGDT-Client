// Parses the html/template files once at startup and renders pages

package view

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/CookieG77/AppGDT-Client/internal/session"
)

// Page is the data given to every template.
// Title is shown in the <title> tag, Description in the meta description
// (a default one is used when empty) and Data holds the page-specific content.
// The other fields are filled by Render from the request.
type Page struct {
	Title       string
	Description string
	Data        any

	// User is the authenticated user, nil for a visitor
	User *session.Current
	// Flash is the one-time message to show, if any
	Flash *session.Flash
	// CSRFToken must be put in every form (see the "csrf" template)
	CSRFToken string
	// Path is the current URL path, used to mark the active link
	Path string
}

// ErrorData is the content of the error page.
type ErrorData struct {
	Status  int
	Message string
}

// Renderer holds one template set per page: the shared layouts and partials
// plus the page itself. Each page defines its own "content" block, so pages
// must be parsed separately to avoid overriding each other.
type Renderer struct {
	pages map[string]*template.Template
}

// NewRenderer parses every template found in fsys under "templates/".
// It fails at startup if a template is invalid, rather than on the first request.
func NewRenderer(fsys fs.FS) (*Renderer, error) {
	base, err := template.New("").Funcs(funcs()).ParseFS(fsys,
		"templates/layouts/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parsing layouts and partials failed: %w", err)
	}

	pageFiles, err := fs.Glob(fsys, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("listing pages failed: %w", err)
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for _, file := range pageFiles {
		name := strings.TrimSuffix(path.Base(file), ".html")

		tmpl, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("cloning base for page %q failed: %w", name, err)
		}
		if _, err := tmpl.ParseFS(fsys, file); err != nil {
			return nil, fmt.Errorf("parsing page %q failed: %w", name, err)
		}
		pages[name] = tmpl
	}

	return &Renderer{pages: pages}, nil
}

// Render writes the given page with the given status code.
// The page is first rendered in a buffer: if a template fails, a clean 500
// page is sent instead of a half-written one.
func (r *Renderer) Render(w http.ResponseWriter, req *http.Request, status int, page string, data Page) {
	tmpl, ok := r.pages[page]
	if !ok {
		slog.Error("unknown page", "page", page)
		writeFallbackError(w)
		return
	}

	ctx := req.Context()
	data.User = session.CurrentFrom(ctx)
	data.Flash = session.FlashFrom(ctx)
	data.CSRFToken = session.CSRFTokenFrom(ctx)
	data.Path = req.URL.Path

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		slog.Error("failed to render page", "page", page, "error", err)
		writeFallbackError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		slog.Warn("failed to write response", "error", err)
	}
}

// RenderError renders the error page with the given status and message.
func (r *Renderer) RenderError(w http.ResponseWriter, req *http.Request, status int, message string) {
	r.Render(w, req, status, "error", Page{
		Title: errorTitle(status),
		Data:  ErrorData{Status: status, Message: message},
	})
}

// errorTitle returns a French title for the most common error statuses.
func errorTitle(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Requête invalide"
	case http.StatusUnauthorized:
		return "Authentification requise"
	case http.StatusForbidden:
		return "Accès refusé"
	case http.StatusNotFound:
		return "Page introuvable"
	case http.StatusRequestEntityTooLarge:
		return "Envoi trop volumineux"
	case http.StatusTooManyRequests:
		return "Trop de requêtes"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return "Service indisponible"
	default:
		return "Erreur interne"
	}
}

// writeFallbackError is used when the error page itself cannot be rendered.
func writeFallbackError(w http.ResponseWriter) {
	http.Error(w, "Une erreur interne est survenue.", http.StatusInternalServerError)
}

// funcs returns the helpers available in every template.
func funcs() template.FuncMap {
	return template.FuncMap{
		"dict": dict,
	}
}

// dict builds a map from key/value pairs, to give several named values to
// a partial template: {{template "field" dict "Name" "email" "Label" "Email"}}
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict expects key/value pairs, got %d values", len(pairs))
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key %v is not a string", pairs[i])
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}
