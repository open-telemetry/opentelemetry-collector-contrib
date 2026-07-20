// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscredentialsproviderextension

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name:        "empty config is rejected",
			config:      Config{},
			expectedErr: errNoCredentialSource,
		},
		{
			name:        "imds endpoint only is rejected",
			config:      Config{IMDSEndpoint: "https://169.254.169.254"},
			expectedErr: errNoCredentialSource,
		},
		{
			name:   "profile only",
			config: Config{Profile: "my-profile"},
		},
		{
			name: "static credentials",
			config: Config{
				Credentials: configoptional.Some(CredentialsConfig{
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
				}),
			},
		},
		{
			name: "static credentials with session token",
			config: Config{
				Credentials: configoptional.Some(CredentialsConfig{
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
					SessionToken:    "TOKEN",
				}),
			},
		},
		{
			name: "assume role",
			config: Config{
				AssumeRole: configoptional.Some(AssumeRoleConfig{
					ARN:        "arn:aws:iam::123456789012:role/monitoring",
					ExternalID: "my-external-id",
					STSRegion:  "us-east-1",
				}),
			},
		},
		{
			name: "static credentials with assume role",
			config: Config{
				Credentials: configoptional.Some(CredentialsConfig{
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
				}),
				AssumeRole: configoptional.Some(AssumeRoleConfig{
					ARN: "arn:aws:iam::123456789012:role/monitoring",
				}),
			},
		},
		{
			name: "profile with assume role",
			config: Config{
				Profile: "my-profile",
				AssumeRole: configoptional.Some(AssumeRoleConfig{
					ARN: "arn:aws:iam::123456789012:role/monitoring",
				}),
			},
		},
		{
			name: "access key without secret",
			config: Config{
				Credentials: configoptional.Some(CredentialsConfig{
					AccessKeyID: "AKID",
				}),
			},
			expectedErr: errEmptyStaticCredentials,
		},
		{
			name: "secret without access key",
			config: Config{
				Credentials: configoptional.Some(CredentialsConfig{
					SecretAccessKey: "SECRET",
				}),
			},
			expectedErr: errEmptyStaticCredentials,
		},
		{
			name: "profile with static credentials",
			config: Config{
				Profile: "my-profile",
				Credentials: configoptional.Some(CredentialsConfig{
					AccessKeyID:     "AKID",
					SecretAccessKey: "SECRET",
				}),
			},
			expectedErr: errProfileWithStaticCredentials,
		},
		{
			name: "assume role without arn",
			config: Config{
				AssumeRole: configoptional.Some(AssumeRoleConfig{
					ExternalID: "my-external-id",
				}),
			},
			expectedErr: errAssumeRoleMissingARN,
		},
		{
			name: "invalid imds endpoint",
			config: Config{
				Profile:      "my-profile",
				IMDSEndpoint: "xyz",
			},
			expectedErr: errors.New("unable to parse URI for imds_endpoint"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
