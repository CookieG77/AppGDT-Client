// Handle the loading of the '.env' variables through custom struct to prevent changes during run

package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    int
	Address string
	API     *APIConfig
	TLS     *TLSConfig
}

type APIConfig struct {
	// BaseURL is the root URL of the AppGDT-Server API, without trailing slash
	BaseURL string
	// Timeout is the maximum duration of a single API call
	Timeout time.Duration
}

// TLSConfig enables HTTPS between the browser and the client when a
// certificate and its private key are given.
type TLSConfig struct {
	CertFile string
	KeyFile  string
	// HSTS asks browsers to always use HTTPS for this host.
	// Off by default: on 'localhost' it would apply to every local port.
	HSTS bool
}

// Enabled reports whether the client must serve HTTPS.
func (t *TLSConfig) Enabled() bool {
	return t.CertFile != "" && t.KeyFile != ""
}

func LoadConfig() (*Config, error) {

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading .env file failed: %w", err)
	}

	apiBaseURL, err := parseBaseURL(getEnvOrDefault("API_BASE_URL", "http://localhost:8080"))
	if err != nil {
		return nil, err
	}

	tlsCfg := &TLSConfig{
		CertFile: getEnvOrDefault("TLS_CERT_FILE", ""),
		KeyFile:  getEnvOrDefault("TLS_KEY_FILE", ""),
		HSTS:     getEnvOrDefaultBool("TLS_HSTS", false),
	}
	if (tlsCfg.CertFile == "") != (tlsCfg.KeyFile == "") {
		return nil, errors.New("environment variables 'TLS_CERT_FILE' and 'TLS_KEY_FILE' must be set together")
	}

	cfg := &Config{
		Port:    getEnvOrDefaultPort("PORT", 3000),
		Address: getEnvOrDefault("ADDRESS", "localhost"),
		API: &APIConfig{
			BaseURL: apiBaseURL,
			Timeout: getEnvOrDefaultDuration("API_TIMEOUT", 5*time.Second),
		},
		TLS: tlsCfg,
	}

	return cfg, nil
}

// parseBaseURL checks that raw is an absolute http(s) URL and returns it
// without trailing slash, so that paths can be appended directly.
func parseBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("environment variable 'API_BASE_URL' must be an absolute http(s) URL, got %q", raw)
	}
	return strings.TrimRight(u.String(), "/"), nil
}
