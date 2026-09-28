package view

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CookieG77/AppGDT-Client/web"
)

func TestExcerpt(t *testing.T) {
	tests := []struct {
		text string
		max  int
		want string
	}{
		{"  court  ", 20, "court"},
		{"un deux trois quatre", 12, "un deux…"},
		{"éééééééééé", 5, "ééééé…"},
		{"ligne 1\nligne 2", 50, "ligne 1\nligne 2"},
	}
	for _, tt := range tests {
		if got := excerpt(tt.text, tt.max); got != tt.want {
			t.Errorf("excerpt(%q, %d) = %q, want %q", tt.text, tt.max, got, tt.want)
		}
	}
}

func TestFrenchDate(t *testing.T) {
	d := time.Date(2026, time.August, 3, 10, 0, 0, 0, time.Local)
	if got := frenchDate(d); got != "3 août 2026" {
		t.Errorf("frenchDate = %q", got)
	}
}

func TestDict(t *testing.T) {
	m, err := dict("Name", "email", "Required", true)
	if err != nil || m["Name"] != "email" || m["Required"] != true {
		t.Errorf("dict = %v, %v", m, err)
	}
	if _, err := dict("odd"); err == nil {
		t.Error("an odd number of values must be an error")
	}
	if _, err := dict(1, "x"); err == nil {
		t.Error("a non-string key must be an error")
	}
}

// Every embedded template must parse, and pages must escape their data.
func TestRendererEscapesData(t *testing.T) {
	r, err := NewRenderer(web.FS)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	rec := httptest.NewRecorder()
	r.RenderError(rec, httptest.NewRequest(http.MethodGet, "/x", nil), http.StatusNotFound, "<script>alert(1)</script>")

	body := rec.Body.String()
	if rec.Code != http.StatusNotFound || !strings.Contains(body, "Page introuvable") {
		t.Fatalf("status %d, body %s", rec.Code, body)
	}
	if strings.Contains(body, "<script>alert(1)") || !strings.Contains(body, "&lt;script&gt;") {
		t.Error("the message must be escaped")
	}

	rec = httptest.NewRecorder()
	r.Render(rec, httptest.NewRequest(http.MethodGet, "/", nil), http.StatusOK, "missing-page", Page{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("an unknown page must give a 500, got %d", rec.Code)
	}
}
