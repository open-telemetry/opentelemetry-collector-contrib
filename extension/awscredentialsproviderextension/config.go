// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package awscredentialsproviderextension // import "github.com/elastic/opentelemetry-collector-components/extension/awscredentialsproviderextension"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

// Config stores the configuration for the AWS Authenticator extension.
//
// At least one of credentials, assume_role, or profile must be set: the extension
// exists to provide an explicit identity, so components that want the default SDK
// credential chain should simply not reference the extension. Static credentials and
// role assumption compose: when both are set, the static credentials are the base
// identity used to assume the role. When only assume_role is set, the default chain
// provides the base identity for the AssumeRole call.
type Config struct {
	// Profile narrows the default credential chain to a named shared-config profile.
	// Mutually exclusive with static credentials.
	Profile string `mapstructure:"profile"`
	// IMDSEndpoint optionally overrides the EC2 IMDS endpoint used by the default chain.
	IMDSEndpoint string `mapstructure:"imds_endpoint"`
	// Credentials holds static AWS credentials, used instead of the default chain.
	Credentials configoptional.Optional[CredentialsConfig] `mapstructure:"credentials"`
	// AssumeRole configures STS role assumption on top of the base credentials.
	AssumeRole configoptional.Optional[AssumeRoleConfig] `mapstructure:"assume_role"`
}

// CredentialsConfig holds static AWS credentials.
type CredentialsConfig struct {
	// AccessKeyID and SecretAccessKey are static AWS credentials. Both are required.
	AccessKeyID     string              `mapstructure:"access_key_id"`
	SecretAccessKey configopaque.String `mapstructure:"secret_access_key"`
	// SessionToken is the token for temporary static credentials, if used.
	SessionToken configopaque.String `mapstructure:"session_token"`
}

// AssumeRoleConfig configures STS role assumption.
type AssumeRoleConfig struct {
	// ARN of the IAM role to assume. Required when assume_role is set.
	ARN string `mapstructure:"arn"`
	// ExternalID to pass in the AssumeRole call, when required by the role's trust policy.
	ExternalID string `mapstructure:"external_id"`
	// SessionName for the assumed-role session. Optional.
	SessionName string `mapstructure:"session_name"`
	// STSRegion is the region for the STS client used to assume the role. When empty,
	// the SDK's default region resolution applies (e.g. AWS_REGION, shared config).
	STSRegion string `mapstructure:"sts_region"`
}

var _ component.Config = (*Config)(nil)

var (
	errNoCredentialSource           = errors.New("at least one of credentials, assume_role, or profile must be set; omit the component's auth option instead to use the default SDK credential chain")
	errEmptyStaticCredentials       = errors.New("credentials requires both access_key_id and secret_access_key")
	errProfileWithStaticCredentials = errors.New("profile and static credentials are mutually exclusive")
	errAssumeRoleMissingARN         = errors.New("assume_role requires arn")
)

// Validate checks that the configuration is valid.
func (cfg *Config) Validate() error {
	if !cfg.Credentials.HasValue() && !cfg.AssumeRole.HasValue() && cfg.Profile == "" {
		return errNoCredentialSource
	}
	if cfg.IMDSEndpoint != "" {
		if _, err := url.ParseRequestURI(cfg.IMDSEndpoint); err != nil {
			return fmt.Errorf("unable to parse URI for imds_endpoint: %w", err)
		}
	}
	if creds := cfg.Credentials.Get(); creds != nil {
		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return errEmptyStaticCredentials
		}
		if cfg.Profile != "" {
			return errProfileWithStaticCredentials
		}
	}
	if role := cfg.AssumeRole.Get(); role != nil {
		if role.ARN == "" {
			return errAssumeRoleMissingARN
		}
	}
	return nil
}
