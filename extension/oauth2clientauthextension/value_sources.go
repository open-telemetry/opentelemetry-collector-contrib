// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

type valueSource interface {
	getValue() (string, error)
}

type rawSource struct {
	value string
}

func (v rawSource) getValue() (string, error) {
	return v.value, nil
}

type fileSource struct {
	path string
}

func (v fileSource) getValue() (string, error) {
	return readCredentialsFile(v.path)
}
