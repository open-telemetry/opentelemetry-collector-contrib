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
