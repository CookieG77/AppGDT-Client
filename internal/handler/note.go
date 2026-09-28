// Note pages: creation from a space, consultation, modification and deletion (FT4 to FT6)

package handler

import (
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

// Limits of the API, checked here too so that the user gets every error at once
const (
	noteTitleMaxLength   = 200
	noteContentMaxLength = 50000
)

var noteFields = []string{"title", "content", "status"}

type NoteHandler struct {
	api      *apiclient.Client
	renderer *view.Renderer
	sessions *session.Manager
}

func NewNoteHandler(api *apiclient.Client, renderer *view.Renderer, sessions *session.Manager) *NoteHandler {
	return &NoteHandler{api: api, renderer: renderer, sessions: sessions}
}

// --- Consultation --------------------------------------------------------------

type notePage struct {
	Note  *apiclient.Note
	Space *apiclient.Space
}

// Show displays a note in reading mode.
func (h *NoteHandler) Show(w http.ResponseWriter, r *http.Request) {
	note, space, ok := h.loadNote(w, r)
	if !ok {
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "note", view.Page{
		Title:      note.Title,
		Data:       notePage{Note: note, Space: space},
		Breadcrumb: noteCrumbs(space, note, ""),
	})
}

// --- Creation and modification (editing mode) ---------------------------------

// noteFormPage is shared by the creation and modification forms.
type noteFormPage struct {
	Form  *Form
	Space *apiclient.Space
	// Note is nil when creating a note
	Note     *apiclient.Note
	Statuses []apiclient.NoteStatus
}

// NewForm shows the creation form. A note is always created from a space,
// which it then belongs to.
func (h *NoteHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	space, ok := h.loadSpace(w, r)
	if !ok {
		return
	}

	form := emptyForm()
	form.Values["status"] = string(apiclient.StatusTodo)
	h.renderForm(w, r, http.StatusOK, noteFormPage{Form: form, Space: space})
}

func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	space, ok := h.loadSpace(w, r)
	if !ok {
		return
	}

	form := newForm(r, noteFields)
	page := noteFormPage{Form: form, Space: space}

	validateNote(form)
	if form.HasErrors() {
		h.renderForm(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	note, err := h.api.CreateNote(r.Context(), token(r), space.ID, noteInput(form))
	if err != nil {
		if handleSessionExpired(w, r, h.sessions, err) {
			return
		}
		if apiclient.IsNotFound(err) {
			apiPageError(w, r, h.renderer, h.sessions, err)
			return
		}
		h.renderForm(w, r, handleAPIError(r, form, err, noteFields), page)
		return
	}

	h.sessions.SetFlash(w, "success", "La note « "+note.Title+" » a été créée.")
	http.Redirect(w, r, noteURL(note.ID), http.StatusSeeOther)
}

func (h *NoteHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	note, space, ok := h.loadNote(w, r)
	if !ok {
		return
	}

	form := emptyForm()
	form.Values["title"] = note.Title
	form.Values["content"] = note.Content
	form.Values["status"] = string(note.Status)
	h.renderForm(w, r, http.StatusOK, noteFormPage{Form: form, Space: space, Note: note})
}

func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	note, space, ok := h.loadNote(w, r)
	if !ok {
		return
	}

	form := newForm(r, noteFields)
	page := noteFormPage{Form: form, Space: space, Note: note}

	validateNote(form)
	if form.HasErrors() {
		h.renderForm(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	updated, err := h.api.UpdateNote(r.Context(), token(r), note.ID, noteInput(form))
	if err != nil {
		if handleSessionExpired(w, r, h.sessions, err) {
			return
		}
		if apiclient.IsNotFound(err) {
			apiPageError(w, r, h.renderer, h.sessions, err)
			return
		}
		h.renderForm(w, r, handleAPIError(r, form, err, noteFields), page)
		return
	}

	h.sessions.SetFlash(w, "success", "La note a été enregistrée.")
	http.Redirect(w, r, noteURL(updated.ID), http.StatusSeeOther)
}

func (h *NoteHandler) renderForm(w http.ResponseWriter, r *http.Request, status int, page noteFormPage) {
	page.Statuses = apiclient.NoteStatuses

	title := "Nouvelle note"
	crumbs := spaceCrumbs(page.Space, title)
	if page.Note != nil {
		title = "Modifier la note"
		crumbs = noteCrumbs(page.Space, page.Note, "Modification")
	}
	h.renderer.Render(w, r, status, "note_form", view.Page{
		Title:      formTitle(title, page.Form),
		Data:       page,
		Breadcrumb: crumbs,
	})
}

func validateNote(form *Form) {
	form.Required(map[string]string{"title": "Donnez un titre à la note."}, "title")
	if utf8.RuneCountInString(form.Get("title")) > noteTitleMaxLength {
		form.AddError("title", "Le titre ne doit pas dépasser 200 caractères.")
	}
	if utf8.RuneCountInString(form.Get("content")) > noteContentMaxLength {
		form.AddError("content", "Le contenu ne doit pas dépasser 50 000 caractères.")
	}
	if !apiclient.NoteStatus(form.Get("status")).Valid() {
		form.AddError("status", "Choisissez l'état de la note.")
	}
}

func noteInput(form *Form) apiclient.NoteInput {
	return apiclient.NoteInput{
		Title:   form.Get("title"),
		Content: form.Get("content"),
		Status:  apiclient.NoteStatus(form.Get("status")),
	}
}

// --- Deletion ------------------------------------------------------------------

func (h *NoteHandler) ConfirmDelete(w http.ResponseWriter, r *http.Request) {
	note, space, ok := h.loadNote(w, r)
	if !ok {
		return
	}

	h.renderer.Render(w, r, http.StatusOK, "confirm_delete", view.Page{
		Title: "Supprimer la note " + note.Title,
		Data: confirmDeletePage{
			Heading:     "Supprimer la note « " + note.Title + " » ?",
			Message:     "La note sera définitivement supprimée. Cette action est irréversible.",
			Action:      noteURL(note.ID) + "/delete",
			ButtonLabel: "Supprimer la note",
			CancelURL:   noteURL(note.ID),
		},
		Breadcrumb: noteCrumbs(space, note, "Suppression"),
	})
}

func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, h.renderer, "noteId")
	if !ok {
		return
	}

	// The note is read first to know which space to go back to
	note, err := h.api.GetNote(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}
	if err := h.api.DeleteNote(r.Context(), token(r), id); err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	h.sessions.SetFlash(w, "success", "La note « "+note.Title+" » a été supprimée.")
	http.Redirect(w, r, spaceURL(note.SpaceID), http.StatusSeeOther)
}

// --- Helpers -------------------------------------------------------------------

// loadSpace reads the space of the URL (/spaces/{spaceId}/…).
func (h *NoteHandler) loadSpace(w http.ResponseWriter, r *http.Request) (*apiclient.Space, bool) {
	id, ok := pathID(w, r, h.renderer, "spaceId")
	if !ok {
		return nil, false
	}
	space, err := h.api.GetSpace(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return nil, false
	}
	return space, true
}

// loadNote reads the note of the URL (/notes/{noteId}/…) and its space,
// whose name is shown in the breadcrumb.
func (h *NoteHandler) loadNote(w http.ResponseWriter, r *http.Request) (*apiclient.Note, *apiclient.Space, bool) {
	id, ok := pathID(w, r, h.renderer, "noteId")
	if !ok {
		return nil, nil, false
	}
	note, err := h.api.GetNote(r.Context(), token(r), id)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return nil, nil, false
	}
	space, err := h.api.GetSpace(r.Context(), token(r), note.SpaceID)
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return nil, nil, false
	}
	return note, space, true
}

func noteURL(id int64) string {
	return "/notes/" + strconv.FormatInt(id, 10)
}

// noteCrumbs builds the breadcrumb of a note page (see spaceCrumbs).
func noteCrumbs(space *apiclient.Space, note *apiclient.Note, current string) []view.Crumb {
	crumbs := []view.Crumb{
		{Label: "Mes espaces", URL: "/spaces"},
		{Label: space.Name, URL: spaceURL(space.ID)},
	}
	if current == "" {
		return append(crumbs, view.Crumb{Label: note.Title})
	}
	return append(crumbs, view.Crumb{Label: note.Title, URL: noteURL(note.ID)}, view.Crumb{Label: current})
}
