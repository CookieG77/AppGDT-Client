// Small helper to parse more easily the environnement values while logging encountered problems while parsing

package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

// getEnvOrDefault returns the requested env variable if present and valid.
// Otherwise, returns the given default value.
func getEnvOrDefault(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	} else if defaultVal != "" {
		slog.Warn("environment variable not set", "key", key, "defaultVal", defaultVal)
	}
	return defaultVal
}

// getEnvOrDefaultInt returns the requested int env variable if present and valid.
// Otherwise, returns the given int default value.
func getEnvOrDefaultInt(key string, defaultVal int) int {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		slog.Warn("environment variable not set", "key", key, "defaultVal", defaultVal)
		return defaultVal
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		slog.Warn("environment variable is not a valid integer", "key", key, "defaultVal", defaultVal)
		return defaultVal
	}
	return i
}

// inRange returns true if 'n' is in [a, b], false otherwise.
func inRange(a int, b int, n int) bool {
	return n >= a && n <= b
}

// getEnvOrDefaultPort returns the requested port env variable if present and valid.
// Otherwise, returns the given port default value.
func getEnvOrDefaultPort(key string, defaultVal int) int {
	if value := getEnvOrDefaultInt(key, defaultVal); inRange(1, 65535, value) {
		return value
	}
	slog.Warn("environment variable is not a valid port", "key", key, "defaultVal", defaultVal)
	return defaultVal
}

// getEnvOrDefaultDuration returns the requested duration env variable if present and valid.
// Otherwise, returns the given duration default value.
func getEnvOrDefaultDuration(key string, defaultVal time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		duration, err := time.ParseDuration(value)
		if err == nil && duration > 0 {
			return duration
		}
		slog.Warn("environment variable is not a valid duration", "key", key, "defaultVal", defaultVal)
	}
	return defaultVal
}
