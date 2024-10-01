// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sigv4authextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/component"
)

// Config stores the configuration for the Sigv4 Authenticator
type Config struct {
	Region        string     `mapstructure:"region,omitempty"`
	Service       string     `mapstructure:"service,omitempty"`
	AssumeRole    AssumeRole `mapstructure:"assume_role"`
	credsProvider *aws.CredentialsProvider
}

// AssumeRole holds the configuration needed to assume a role
type AssumeRole struct {
	ARN         string `mapstructure:"arn,omitempty"`
	SessionName string `mapstructure:"session_name,omitempty"`
	STSRegion   string `mapstructure:"sts_region,omitempty"`
}

// compile time check that the Config struct satisfies the component.Config interface
var _ component.Config = (*Config)(nil)

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
