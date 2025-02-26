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
	Region                    string                             `mapstructure:"region,omitempty"`
	Service                   string                             `mapstructure:"service,omitempty"`
	AssumeRole                *AssumeRoleSettings                `mapstructure:"assume_role"`
	AssumeRoleWithWebIdentity *AssumeRoleWithWebIdentitySettings `mapstructure:"assume_role_with_web_identity,omitempty"`
	credsProvider             *aws.CredentialsProvider
}

// AssumeRole holds the configuration needed to assume a role
type AssumeRoleSettings struct {
	ARN         string `mapstructure:"arn,omitempty"`
	SessionName string `mapstructure:"session_name,omitempty"`
	STSRegion   string `mapstructure:"sts_region,omitempty"`
}

// AssumeRoleWithWebIdentity holds the configuration needed to assume a role
type AssumeRoleWithWebIdentitySettings struct {
	ARN       string `mapstructure:"arn,omitempty"`
	TokenFile string `mapstructure:"token_file"`
	STSRegion string `mapstructure:"sts_region,omitempty"`
}

// compile time check that the Config struct satisfies the component.Config interface
var _ component.Config = (*Config)(nil)

// Validate checks that the configuration is valid.
// We aim to catch most errors here to ensure that we
// fail early and to avoid revalidating static data.
func (cfg *Config) Validate() error {
	assumeRole := cfg.AssumeRole != nil
	assumeRoleWithWebIdentity := cfg.AssumeRoleWithWebIdentity != nil

	switch {
	case !assumeRoleWithWebIdentity:
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
	case !assumeRole && assumeRoleWithWebIdentity:
		if cfg.AssumeRoleWithWebIdentity.STSRegion == "" && cfg.Region != "" {
			cfg.AssumeRoleWithWebIdentity.STSRegion = cfg.Region
		}

		credsProvider, err := getCredsProviderFromWebIdentityConfig(cfg)
		if err != nil {
			return fmt.Errorf("could not retrieve credential provider: %w", err)
		}
		if credsProvider == nil {
			return fmt.Errorf("credsProvider cannot be nil")
		}
		cfg.credsProvider = credsProvider
	case assumeRole && assumeRoleWithWebIdentity:
		return fmt.Errorf("both assume_role and assume_role_with_web_identity were defined in the config - only define one or the other")
	}
	return nil
}
