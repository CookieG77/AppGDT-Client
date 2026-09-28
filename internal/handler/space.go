// Space pages: list, creation, consultation, modification and deletion (FT2)

package handler

import (
	"net/http"
	"unicode/utf8"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

// Limits of the API, checked here too so that the user gets every error at once
const (
	spaceNameMaxLength        = 100
	spaceDescriptionMaxLength = 1000
)

var spaceFields = []string{"name", "description"}

type SpaceHandler struct {
	api      *apiclient.Client
	renderer *view.Renderer
	sessions *session.Manager
}

func NewSpaceHandler(api *apiclient.Client, renderer *view.Renderer, sessions *session.Manager) *SpaceHandler {
	return &SpaceHandler{api: api, renderer: renderer, sessions: sessions}
}

// --- List ----------------------------------------------------------------------

type spaceListPage struct {
	Spaces []apiclient.Space
}

// List shows the spaces of the user.
func (h *SpaceHandler) List(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.api.ListSpaces(r.Context(), token(r))
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "spaces", view.Page{
		Title: "Mes espaces",
		Data:  spaceListPage{Spaces: spaces},
	})
}

// --- Consultation --------------------------------------------------------------

type spacePage struct {
	Space *apiclient.Space
	// Notes are the notes shown, after the status filter
	Notes []apiclient.Note
	// Total is the number of notes of the space, whatever the filter
	Total int
	// Counts gives the number of notes of each status, in display order
	Counts []statusCount
	// Filter is the status selected with ?status=…, empty for all notes
	Filter apiclient.NoteStatus
}

type statusCount struct {
	Status apiclient.NoteStatus
	Count  int
}

// Show displays a space and all its notes (FT3).
func (h *SpaceHandler) Show(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return
	}

	space, err := h.api.GetSpace(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}
	notes, err := h.api.ListNotes(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "space", view.Page{
		Title:      space.Name,
		Data:       spacePageData(space, notes, apiclient.NoteStatus(r.URL.Query().Get("status"))),
		Breadcrumb: spaceCrumbs(space, ""),
	})
}

// --- Creation and modification -----------------------------------------------

// spaceFormPage is shared by the creation and modification forms.
type spaceFormPage struct {
	Form *Form
	// Space is nil when creating a space
	Space *apiclient.Space
}

func (h *SpaceHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	h.renderForm(w, r, http.StatusOK, spaceFormPage{Form: emptyForm()})
}

func (h *SpaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	form := newForm(r, spaceFields)
	page := spaceFormPage{Form: form}

	validateSpace(form)
	if form.HasErrors() {
		h.renderForm(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	space, err := h.api.CreateSpace(r.Context(), token(r), spaceInput(form))
	if err != nil {
		if handleSessionExpired(w, r, h.sessions, err) {
			return
		}
		h.renderForm(w, r, handleAPIError(r, form, err, spaceFields), page)
		return
	}

	h.sessions.SetFlash(w, "success", "L'espace « "+space.Name+" » a été créé.")
	http.Redirect(w, r, spaceURL(space.ID), http.StatusSeeOther)
}

func (h *SpaceHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return
	}

	space, err := h.api.GetSpace(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	form := emptyForm()
	form.Values["name"] = space.Name
	form.Values["description"] = space.Description
	h.renderForm(w, r, http.StatusOK, spaceFormPage{Form: form, Space: space})
}

func (h *SpaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return
	}

	form := newForm(r, spaceFields)
	// Only the ID is needed to build the page: the breadcrumb shows the
	// name being typed
	name := form.Get("name")
	if name == "" {
		name = "Espace"
	}
	page := spaceFormPage{Form: form, Space: &apiclient.Space{ID: id, Name: name}}

	validateSpace(form)
	if form.HasErrors() {
		h.renderForm(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	space, err := h.api.UpdateSpace(r.Context(), token(r), id, spaceInput(form))
	if err != nil {
		if handleSessionExpired(w, r, h.sessions, err) {
			return
		}
		if apiclient.IsNotFound(err) {
			apiPageError(w, r, h.renderer, h.sessions, err)
			return
		}
		h.renderForm(w, r, handleAPIError(r, form, err, spaceFields), page)
		return
	}

	h.sessions.SetFlash(w, "success", "L'espace « "+space.Name+" » a été modifié.")
	http.Redirect(w, r, spaceURL(space.ID), http.StatusSeeOther)
}

func (h *SpaceHandler) renderForm(w http.ResponseWriter, r *http.Request, status int, page spaceFormPage) {
	title := "Nouvel espace"
	crumbs := []view.Crumb{{Label: "Mes espaces", URL: "/spaces"}, {Label: title}}
	if page.Space != nil {
		title = "Modifier l'espace"
		crumbs = spaceCrumbs(page.Space, "Modification")
	}
	h.renderer.Render(w, r, status, "space_form", view.Page{
		Title:      formTitle(title, page.Form),
		Data:       page,
		Breadcrumb: crumbs,
	})
}

func validateSpace(form *Form) {
	form.Required(map[string]string{"name": "Donnez un nom à l'espace."}, "name")
	if utf8.RuneCountInString(form.Get("name")) > spaceNameMaxLength {
		form.AddError("name", "Le nom ne doit pas dépasser 100 caractères.")
	}
	if utf8.RuneCountInString(form.Get("description")) > spaceDescriptionMaxLength {
		form.AddError("description", "La description ne doit pas dépasser 1000 caractères.")
	}
}

func spaceInput(form *Form) apiclient.SpaceInput {
	return apiclient.SpaceInput{Name: form.Get("name"), Description: form.Get("description")}
}

// --- Deletion ------------------------------------------------------------------

// ConfirmDelete asks for a confirmation before deleting: it works without
// JavaScript and leaves time to cancel, since the notes are deleted too.
func (h *SpaceHandler) ConfirmDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return
	}

	space, err := h.api.GetSpace(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "confirm_delete", view.Page{
		Title: "Supprimer l'espace " + space.Name,
		Data: confirmDeletePage{
			Heading:     "Supprimer l'espace « " + space.Name + " » ?",
			Message:     "L'espace et toutes les notes qu'il contient seront définitivement supprimés. Cette action est irréversible.",
			Action:      spaceURL(space.ID) + "/delete",
			ButtonLabel: "Supprimer l'espace",
			CancelURL:   spaceURL(space.ID),
		},
		Breadcrumb: spaceCrumbs(space, "Suppression"),
	})
}

func (h *SpaceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return
	}

	if err := h.api.DeleteSpace(r.Context(), token(r), id); err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	h.sessions.SetFlash(w, "success", "L'espace a été supprimé.")
	http.Redirect(w, r, "/spaces", http.StatusSeeOther)
}

// spaceCrumbs builds the breadcrumb of a page inside a space. The last
// label is the current page; without it, the space itself is the current page.
func spaceCrumbs(space *apiclient.Space, current string) []view.Crumb {
	crumbs := []view.Crumb{{Label: "Mes espaces", URL: "/spaces"}}
	if current == "" {
		return append(crumbs, view.Crumb{Label: space.Name})
	}
	return append(crumbs, view.Crumb{Label: space.Name, URL: spaceURL(space.ID)}, view.Crumb{Label: current})
}

// countByStatus counts the notes of each status.
func countByStatus(notes []apiclient.Note) []statusCount {
	counts := make([]statusCount, len(apiclient.NoteStatuses))
	for i, status := range apiclient.NoteStatuses {
		counts[i].Status = status
		for _, n := range notes {
			if n.Status == status {
				counts[i].Count++
			}
		}
	}
	return counts
}

// spacePageData keeps only the notes of the requested status. The filter is
// applied here rather than by the API: a space holds few notes, and the API
// contract stays unchanged. An unknown status shows every note.
func spacePageData(space *apiclient.Space, notes []apiclient.Note, filter apiclient.NoteStatus) spacePage {
	page := spacePage{Space: space, Notes: notes, Total: len(notes), Counts: countByStatus(notes)}
	if !filter.Valid() {
		return page
	}
	page.Filter = filter
	page.Notes = nil
	for _, n := range notes {
		if n.Status == filter {
			page.Notes = append(page.Notes, n)
		}
	}
	return page
}
