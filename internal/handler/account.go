// Account page: profile, data export (right to data portability) and account
// deletion (right to erasure)

package handler

import (
	"mime"
	"net/http"
	"strconv"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
	"github.com/CookieG77/AppGDT-Client/internal/session"
	"github.com/CookieG77/AppGDT-Client/internal/view"
)

var deleteAccountFields = []string{"password", "confirm"}

type AccountHandler struct {
	api      *apiclient.Client
	renderer *view.Renderer
	sessions *session.Manager
}

func NewAccountHandler(api *apiclient.Client, renderer *view.Renderer, sessions *session.Manager) *AccountHandler {
	return &AccountHandler{api: api, renderer: renderer, sessions: sessions}
}

// Show displays the profile and the actions on personal data.
func (h *AccountHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.renderer.Render(w, r, http.StatusOK, "account", view.Page{
		Title:      "Mon compte",
		Breadcrumb: []view.Crumb{{Label: "Mon compte"}},
	})
}

// Export sends every data of the user as a JSON file to download.
func (h *AccountHandler) Export(w http.ResponseWriter, r *http.Request) {
	export, err := h.api.ExportAccount(r.Context(), token(r))
	if err != nil {
		apiPageError(w, r, h.renderer, h.sessions, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": export.Filename}))
	w.Header().Set("Content-Length", strconv.Itoa(len(export.Body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(export.Body)
}

type deleteAccountPage struct {
	Form *Form
}

// DeleteForm asks for the password and an explicit confirmation.
func (h *AccountHandler) DeleteForm(w http.ResponseWriter, r *http.Request) {
	h.renderDelete(w, r, http.StatusOK, deleteAccountPage{Form: emptyForm()})
}

// Delete removes the account and all its data. The API requires the
// password, so that a stolen session is not enough to erase an account.
func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	form := newForm(r, deleteAccountFields, "password")
	page := deleteAccountPage{Form: form}

	form.Required(map[string]string{
		"password": "Saisissez votre mot de passe pour confirmer.",
		"confirm":  "Cochez la case pour confirmer la suppression définitive.",
	}, deleteAccountFields...)
	if form.HasErrors() {
		h.renderDelete(w, r, http.StatusUnprocessableEntity, page)
		return
	}

	err := h.api.DeleteAccount(r.Context(), token(r), apiclient.DeleteAccountInput{Password: form.Get("password")})
	if err != nil {
		if handleSessionExpired(w, r, h.sessions, err) {
			return
		}
		var status int
		if apiErr, ok := apiclient.AsAPIError(err); ok && apiErr.Status == http.StatusForbidden {
			form.AddError("password", "Mot de passe incorrect.")
			status = http.StatusForbidden
		} else {
			status = handleAPIError(r, form, err, deleteAccountFields)
		}
		h.renderDelete(w, r, status, page)
		return
	}

	h.sessions.ClearToken(w)
	h.sessions.SetFlash(w, "success", "Votre compte et toutes vos données ont été définitivement supprimés.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AccountHandler) renderDelete(w http.ResponseWriter, r *http.Request, status int, page deleteAccountPage) {
	h.renderer.Render(w, r, status, "account_delete", view.Page{
		Title:      formTitle("Supprimer mon compte", page.Form),
		Data:       page,
		Breadcrumb: []view.Crumb{{Label: "Mon compte", URL: "/account"}, {Label: "Suppression du compte"}},
	})
}
