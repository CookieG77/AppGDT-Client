// Handle the loading of the '.env' variables through custom struct to prevent changes during run

package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port    int
	Address string
	API     *APIConfig
}

type APIConfig struct {
	// BaseURL is the root URL of the AppGDT-Server API, without trailing slash
	BaseURL string
}

func LoadConfig() (*Config, error) {

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading .env file failed: %w", err)
	}

	apiBaseURL, err := parseBaseURL(getEnvOrDefault("API_BASE_URL", "http://localhost:8080"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:    getEnvOrDefaultPort("PORT", 3000),
		Address: getEnvOrDefault("ADDRESS", "localhost"),
		API: &APIConfig{
			BaseURL: apiBaseURL,
		},
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
