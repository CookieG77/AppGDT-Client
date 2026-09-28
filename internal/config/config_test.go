package config

import "testing"

func TestParseBaseURL(t *testing.T) {
	ok := map[string]string{
		"http://localhost:8080":      "http://localhost:8080",
		"https://api.example.com/":   "https://api.example.com",
		"https://api.example.com/v1": "https://api.example.com/v1",
	}
	for raw, want := range ok {
		if got, err := parseBaseURL(raw); err != nil || got != want {
			t.Errorf("parseBaseURL(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "localhost:8080", "ftp://x", "/api", "http://"} {
		if _, err := parseBaseURL(raw); err == nil {
			t.Errorf("parseBaseURL(%q) must fail", raw)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("API_BASE_URL", "https://localhost:8443")
	t.Setenv("PORT", "4000")
	t.Setenv("API_TIMEOUT", "2s")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Port != 4000 || cfg.API.BaseURL != "https://localhost:8443" || cfg.API.Timeout.String() != "2s" || cfg.TLS.Enabled() {
		t.Errorf("unexpected config %+v %+v %+v", cfg, cfg.API, cfg.TLS)
	}

	// Invalid values fall back to their default
	t.Setenv("PORT", "99999")
	t.Setenv("API_TIMEOUT", "bientôt")
	cfg, _ = LoadConfig()
	if cfg.Port != 3000 || cfg.API.Timeout.String() != "5s" {
		t.Errorf("defaults expected, got port %d, timeout %s", cfg.Port, cfg.API.Timeout)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := map[string]map[string]string{
		"TLS cert without key": {"TLS_CERT_FILE": "cert.pem"},
		"CA file with http":    {"API_BASE_URL": "http://localhost:8080", "API_CA_FILE": "ca.pem"},
		"invalid API URL":      {"API_BASE_URL": "localhost"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := LoadConfig(); err == nil {
				t.Error("LoadConfig must fail")
			}
		})
	}
}
