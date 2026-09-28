// Authentication and account routes

package apiclient

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// Register creates an account. The user must log in afterwards.
func (c *Client) Register(ctx context.Context, in RegisterInput) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodPost, "/auth/register", "", in, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// Login checks the credentials and returns a JWT.
func (c *Client) Login(ctx context.Context, in LoginInput) (*AuthToken, error) {
	var token AuthToken
	if err := c.do(ctx, http.MethodPost, "/auth/login", "", in, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// Me returns the profile of the user owning the token.
func (c *Client) Me(ctx context.Context, token string) (*User, error) {
	var user User
	if err := c.do(ctx, http.MethodGet, "/users/me", token, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// DeleteAccount deletes the account and all its data (right to erasure).
// The current password is required by the API.
func (c *Client) DeleteAccount(ctx context.Context, token string, in DeleteAccountInput) error {
	return c.do(ctx, http.MethodDelete, "/users/me", token, in, nil)
}

// ExportAccount returns every data of the user as a JSON file to download
// (right to data portability).
func (c *Client) ExportAccount(ctx context.Context, token string) (*AccountExport, error) {
	resp, err := c.send(ctx, http.MethodGet, "/users/me/export", token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading account export failed: %w", err)
	}

	filename := "gdt-export.json"
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
		filename = params["filename"]
	}

	return &AccountExport{Filename: filename, Body: body}, nil
}
