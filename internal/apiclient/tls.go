package apiclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// LoadTLSConfig builds the TLS settings used to reach the API over HTTPS.
// The certificate authorities of the system are always trusted; caFile
// (optional) adds more, such as the local authority created by mkcert or a
// self-signed certificate of the API. Certificates are always verified.
func LoadTLSConfig(caFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return cfg, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading API_CA_FILE failed: %w", err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("API_CA_FILE contains no valid PEM certificate")
	}
	cfg.RootCAs = pool
	return cfg, nil
}
