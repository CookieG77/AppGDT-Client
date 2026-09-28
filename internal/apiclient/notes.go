// Note routes

package apiclient

import (
	"context"
	"net/http"
	"strconv"
)

func notePath(id int64) string {
	return "/notes/" + strconv.FormatInt(id, 10)
}

// ListNotes returns the notes of a space.
func (c *Client) ListNotes(ctx context.Context, token string, spaceID int64) ([]Note, error) {
	var notes []Note
	if err := c.do(ctx, http.MethodGet, spacePath(spaceID)+"/notes", token, nil, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// CreateNote creates a note in the given space.
func (c *Client) CreateNote(ctx context.Context, token string, spaceID int64, in NoteInput) (*Note, error) {
	var note Note
	if err := c.do(ctx, http.MethodPost, spacePath(spaceID)+"/notes", token, in, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

// GetNote returns a note of the user.
func (c *Client) GetNote(ctx context.Context, token string, id int64) (*Note, error) {
	var note Note
	if err := c.do(ctx, http.MethodGet, notePath(id), token, nil, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

// UpdateNote replaces the title, content and status of a note.
func (c *Client) UpdateNote(ctx context.Context, token string, id int64, in NoteInput) (*Note, error) {
	var note Note
	if err := c.do(ctx, http.MethodPut, notePath(id), token, in, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

// DeleteNote deletes a note.
func (c *Client) DeleteNote(ctx context.Context, token string, id int64) error {
	return c.do(ctx, http.MethodDelete, notePath(id), token, nil, nil)
}
