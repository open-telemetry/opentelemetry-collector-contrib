// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"fmt"
	"os"
	"strings"
)

type valueSource interface {
	getValue() (string, error)
}

type rawSource struct {
	value string
}

func (v rawSource) getValue() (string, error) {
	return v.value, nil
}

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

type fileSource struct {
	path string
}

func (v fileSource) getValue() (string, error) {
	return readCredentialsFile(v.path)
}
