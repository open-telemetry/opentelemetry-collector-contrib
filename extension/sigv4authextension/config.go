// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/config"
)

// Config stores the configuration for the Sigv4 Authenticator
type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`
	Region                   string     `mapstructure:"region,omitempty"`
	Service                  string     `mapstructure:"service,omitempty"`
	AssumeRole               AssumeRole `mapstructure:"assume_role"`
	credsProvider            *aws.CredentialsProvider
}

// AssumeRole holds the configuration needed to assume a role
type AssumeRole struct {
	ARN         string `mapstructure:"arn,omitempty"`
	SessionName string `mapstructure:"session_name,omitempty"`
	STSRegion   string `mapstructure:"sts_region,omitempty"`
}

// compile time check that the Config struct satisfies the config.Extension interface
var _ config.Extension = (*Config)(nil)

// Validate checks that the configuration is valid.
// We aim to catch most errors here to ensure that we
// fail early and to avoid revalidating static data.
func (cfg *Config) Validate() error {
	if cfg.AssumeRole.STSRegion == "" && cfg.Region != "" {
		cfg.AssumeRole.STSRegion = cfg.Region
	}

	credsProvider, err := getCredsProviderFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("could not retrieve credential provider: %w", err)
	}
	if credsProvider == nil {
		return fmt.Errorf("credsProvider cannot be nil")
	}
	cfg.credsProvider = credsProvider

	return nil
}
