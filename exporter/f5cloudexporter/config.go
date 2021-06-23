// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package f5cloudexporter

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
