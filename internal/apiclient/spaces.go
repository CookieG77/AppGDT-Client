// Space routes

package apiclient

import (
	"context"
	"net/http"
	"strconv"
)

func spacePath(id int64) string {
	return "/spaces/" + strconv.FormatInt(id, 10)
}

// ListSpaces returns the spaces of the user.
func (c *Client) ListSpaces(ctx context.Context, token string) ([]Space, error) {
	var spaces []Space
	if err := c.do(ctx, http.MethodGet, "/spaces", token, nil, &spaces); err != nil {
		return nil, err
	}
	return spaces, nil
}

// CreateSpace creates a space owned by the user.
func (c *Client) CreateSpace(ctx context.Context, token string, in SpaceInput) (*Space, error) {
	var space Space
	if err := c.do(ctx, http.MethodPost, "/spaces", token, in, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// GetSpace returns a space of the user.
func (c *Client) GetSpace(ctx context.Context, token string, id int64) (*Space, error) {
	var space Space
	if err := c.do(ctx, http.MethodGet, spacePath(id), token, nil, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// UpdateSpace replaces the name and description of a space.
func (c *Client) UpdateSpace(ctx context.Context, token string, id int64, in SpaceInput) (*Space, error) {
	var space Space
	if err := c.do(ctx, http.MethodPut, spacePath(id), token, in, &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// DeleteSpace deletes a space and all its notes.
func (c *Client) DeleteSpace(ctx context.Context, token string, id int64) error {
	return c.do(ctx, http.MethodDelete, spacePath(id), token, nil, nil)
}
