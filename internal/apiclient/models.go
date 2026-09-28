// Types exchanged with the API, as defined by the OpenAPI contract of AppGDT-Server

package apiclient

import "time"

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Space struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Note struct {
	ID        int64      `json:"id"`
	SpaceID   int64      `json:"spaceId"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Status    NoteStatus `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// NoteStatus is the state of a note: todo, in_progress or done.
type NoteStatus string

const (
	StatusTodo       NoteStatus = "todo"
	StatusInProgress NoteStatus = "in_progress"
	StatusDone       NoteStatus = "done"
)

// NoteStatuses lists every status, in display order (used for <select>).
var NoteStatuses = []NoteStatus{StatusTodo, StatusInProgress, StatusDone}

// Valid reports whether s is one of the statuses accepted by the API.
func (s NoteStatus) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Label returns the French label shown to the user.
func (s NoteStatus) Label() string {
	switch s {
	case StatusTodo:
		return "Non fait"
	case StatusInProgress:
		return "En cours"
	case StatusDone:
		return "Terminé"
	}
	return string(s)
}

// AuthToken is the answer of a successful login.
type AuthToken struct {
	Token     string `json:"token"`
	TokenType string `json:"tokenType"`
	// ExpiresIn is the token lifetime, in seconds
	ExpiresIn int `json:"expiresIn"`
}

// Lifetime returns the token lifetime as a duration.
func (t AuthToken) Lifetime() time.Duration {
	return time.Duration(t.ExpiresIn) * time.Second
}

type RegisterInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type DeleteAccountInput struct {
	Password string `json:"password"`
}

type SpaceInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NoteInput struct {
	Title   string     `json:"title"`
	Content string     `json:"content"`
	Status  NoteStatus `json:"status"`
}

// AccountExport is a downloadable copy of every data of the user
// (right to data portability). The body is kept as-is from the API.
type AccountExport struct {
	Filename string
	Body     []byte
}
