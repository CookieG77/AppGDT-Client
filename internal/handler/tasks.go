// Task lists in notes: "- [ ] à faire" / "- [x] fait" lines, rendered as
// checkboxes by the Markdown script, can be ticked from the reading mode.

package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/CookieG77/AppGDT-Client/internal/apiclient"
)

// taskLine matches a task list item, as rendered by marked (GFM): an
// optional quote, a list marker ("-", "*", "+" or "1." / "1)"), then "[ ]",
// "[x]" or "[X]" followed by a space or the end of the line.
var taskLine = regexp.MustCompile(`^((?:[ \t]*>)*[ \t]*(?:[-*+]|\d{1,9}[.)])[ \t]+)\[([ xX])\]([ \t]|$)`)

// setTask checks or unchecks the task number index (from 0, in the order of
// the text) and returns the new content. Lines inside fenced code blocks
// are skipped: marked does not render them as checkboxes either.
// It reports false if the note has no such task.
func setTask(content string, index int, done bool) (string, bool) {
	lines := strings.Split(content, "\n")
	fence := ""
	count := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}

		m := taskLine.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		if count == index {
			mark := " "
			if done {
				mark = "x"
			}
			// m[4]:m[5] is the character between the brackets
			lines[i] = line[:m[4]] + mark + line[m[5]:]
			return strings.Join(lines, "\n"), true
		}
		count++
	}
	return content, false
}

// ToggleTask saves the state of one checkbox of a note. It is called by the
// Markdown script (fetch) and answers 204 without a page.
func (h *NoteHandler) ToggleTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("noteId"), 10, 64)
	index, errIndex := strconv.Atoi(r.PostFormValue("index"))
	if err != nil || id <= 0 || errIndex != nil || index < 0 {
		http.Error(w, "Requête invalide.", http.StatusBadRequest)
		return
	}
	done := r.PostFormValue("done") == "true"

	note, err := h.api.GetNote(r.Context(), token(r), id)
	if err != nil {
		taskError(w, r, err)
		return
	}
	content, ok := setTask(note.Content, index, done)
	if !ok {
		http.Error(w, "Cette tâche n'existe pas.", http.StatusBadRequest)
		return
	}
	if content != note.Content {
		input := apiclient.NoteInput{Title: note.Title, Content: content, Status: note.Status}
		if _, err := h.api.UpdateNote(r.Context(), token(r), id, input); err != nil {
			taskError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// taskError answers a failed API call with a short status for the script.
func taskError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case apiclient.IsUnauthorized(err):
		http.Error(w, "Session expirée.", http.StatusUnauthorized)
	case apiclient.IsNotFound(err):
		http.Error(w, "Note introuvable.", http.StatusNotFound)
	case errors.Is(err, apiclient.ErrUnavailable):
		http.Error(w, "Service indisponible.", http.StatusServiceUnavailable)
	default:
		slog.ErrorContext(r.Context(), "unexpected API error", "error", err, "path", r.URL.Path)
		http.Error(w, "Erreur interne.", http.StatusInternalServerError)
	}
}
