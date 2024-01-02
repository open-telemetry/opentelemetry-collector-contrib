// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package f5cloudexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter"

import (
	"fmt"
	"net/url"
	"os"

	otlphttp "go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

// Config defines configuration for F5 Cloud exporter.
type Config struct {
	// Config represents the OTLP HTTP Exporter configuration.
	otlphttp.Config `mapstructure:",squash"`

	// Source represents a unique identifier that is used to distinguish where this data is coming from.
	Source string `mapstructure:"source"`

	// AuthConfig represents the F5 Cloud authentication configuration options.
	AuthConfig AuthConfig `mapstructure:"f5cloud_auth"`
}

func (c *Config) sanitize() error {
	if len(c.Endpoint) == 0 {
		return fmt.Errorf("missing required \"endpoint\" setting")
	}

	endpointURL, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	if len(c.Source) == 0 {
		return fmt.Errorf("missing required \"source\" setting")
	}

	if len(c.AuthConfig.CredentialFile) == 0 {
		return fmt.Errorf("missing required \"f5cloud_auth.credential_file\" setting")
	}

	if _, err := os.Stat(c.AuthConfig.CredentialFile); os.IsNotExist(err) {
		return fmt.Errorf("the provided \"f5cloud_auth.credential_file\" does not exist")
	}

	if len(c.AuthConfig.Audience) == 0 {
		c.AuthConfig.Audience = fmt.Sprintf("%s://%s", endpointURL.Scheme, endpointURL.Hostname())
	}

	return nil
}

// AuthConfig defines F5 Cloud authentication configurations for F5CloudAuthRoundTripper
type AuthConfig struct {
	// CredentialFile is the F5 Cloud credentials for your designated account.
	CredentialFile string `mapstructure:"credential_file"`

	// Audience is the F5 Cloud audience for your designated account.
	Audience string `mapstructure:"audience"`
}
