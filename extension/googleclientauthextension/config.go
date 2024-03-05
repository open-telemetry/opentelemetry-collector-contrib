// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"

// Config defines configuration for the Google client auth extension.
type Config struct {
}

func (cfg *Config) Validate() error {
	return nil
}
