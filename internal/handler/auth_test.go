package handler

import "testing"

func TestSafeNext(t *testing.T) {
	tests := map[string]string{
		"":                     "/spaces",
		"/spaces":              "/spaces",
		"/spaces/3?tab=notes":  "/spaces/3?tab=notes",
		"https://evil.example": "/spaces",
		"//evil.example":       "/spaces",
		"/\\evil.example":      "/spaces",
		"javascript:alert(1)":  "/spaces",
		"spaces":               "/spaces",
		"/%2F%2Fevil.example":  "/%2F%2Fevil.example",
	}
	for next, want := range tests {
		if got := safeNext(next); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", next, got, want)
		}
	}
}
