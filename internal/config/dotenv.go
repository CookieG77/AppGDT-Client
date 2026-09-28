// Minimal '.env' loader, so the client only depends on the standard library

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadDotEnv reads KEY=VALUE lines from the given file and sets them as
// environment variables. Variables already set in the environment are kept,
// so the real environment always wins over the file.
// Empty lines and lines starting with '#' are ignored, and values may be
// wrapped in single or double quotes.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return fmt.Errorf("%s:%d: expected KEY=VALUE", path, lineNum)
		}
		value = unquote(strings.TrimSpace(value))

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNum, err)
		}
	}
	return scanner.Err()
}

// unquote removes one pair of matching surrounding quotes, if any.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
