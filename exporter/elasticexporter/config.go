// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticexporter

import (
	"errors"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines configuration for Elastic APM exporter.
type Config struct {
	config.ExporterSettings    `mapstructure:",squash"`
	configtls.TLSClientSetting `mapstructure:",squash"`

	// APMServerURLs holds the APM Server URL.
	//
	// This is required.
	APMServerURL string `mapstructure:"apm_server_url"`

	// APIKey holds an optional API Key for authorization.
	//
	// https://www.elastic.co/guide/en/apm/server/7.7/api-key-settings.html
	APIKey string `mapstructure:"api_key"`

	// SecretToken holds the optional secret token for authorization.
	//
	// https://www.elastic.co/guide/en/apm/server/7.7/secret-token.html
	SecretToken string `mapstructure:"secret_token"`
}

// Validate validates the configuration.
func (cfg Config) Validate() error {
	if cfg.APMServerURL == "" {
		return errors.New("APMServerURL must be specified")
	}
	return nil
}
