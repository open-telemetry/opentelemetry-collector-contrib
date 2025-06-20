// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func readCredentialsFile(path string) (string, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file %q: %w", path, err)
	}

	credential := strings.TrimSpace(string(f))
	if credential == "" {
		return "", fmt.Errorf("empty credentials file %q", path)
	}
	return credential, nil
}

func getActualValue(value, filepath string) (string, error) {
	if len(filepath) > 0 {
		return readCredentialsFile(filepath)
	}

	return value, nil
}

func createTransport(cfg *Config) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	tlsCfg, err := cfg.TLS.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, err
	}
	transport.TLSClientConfig = tlsCfg
	return transport, nil
}
