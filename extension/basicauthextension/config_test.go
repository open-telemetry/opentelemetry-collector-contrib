// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr bool
	}{
		{
			id:          component.NewID(metadata.Type),
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "server"),
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					Inline: "username1:password1\nusername2:password2\n",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "client"),
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					Username: "username",
					Password: "password",
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "both"),
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "client_aws"),
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					AWSSecret: &AWSSecretClientConfig{
						SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-creds",
						Region:          "us-east-1",
						UsernameKey:     "username",
						PasswordKey:     "password",
						RefreshInterval: 5 * time.Minute,
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "server_aws"),
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					AWSSecret: &AWSSecretHtpasswdConfig{
						SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-htpasswd",
						Region:          "us-east-1",
						RefreshInterval: 10 * time.Minute,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr {
				assert.Error(t, xconfmap.Validate(cfg))
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate_AWSSecretMutualExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
		err  error
	}{
		{
			name: "client_aws_with_inline",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					Username: "user",
					AWSSecret: &AWSSecretClientConfig{
						SecretARN: "arn", Region: "us-east-1",
						UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errAWSSecretAndOtherSource,
		},
		{
			name: "client_aws_with_file",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					PasswordFile: "/path",
					AWSSecret: &AWSSecretClientConfig{
						SecretARN: "arn", Region: "us-east-1",
						UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errAWSSecretAndOtherSource,
		},
		{
			name: "server_aws_with_file",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					File: "/path",
					AWSSecret: &AWSSecretHtpasswdConfig{
						SecretARN: "arn", Region: "us-east-1",
					},
				},
			},
			err: errAWSSecretAndOtherSource,
		},
		{
			name: "server_aws_with_inline",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					Inline: "user:pass",
					AWSSecret: &AWSSecretHtpasswdConfig{
						SecretARN: "arn", Region: "us-east-1",
					},
				},
			},
			err: errAWSSecretAndOtherSource,
		},
		{
			name: "client_aws_missing_arn",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					AWSSecret: &AWSSecretClientConfig{
						Region: "us-east-1", UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errAWSSecretMissingARN,
		},
		{
			name: "client_aws_missing_region",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					AWSSecret: &AWSSecretClientConfig{
						SecretARN: "arn", UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errAWSSecretMissingRegion,
		},
		{
			name: "client_aws_missing_keys",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					AWSSecret: &AWSSecretClientConfig{
						SecretARN: "arn", Region: "us-east-1",
					},
				},
			},
			err: errAWSSecretMissingKeys,
		},
		{
			name: "client_aws_negative_interval",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					AWSSecret: &AWSSecretClientConfig{
						SecretARN: "arn", Region: "us-east-1",
						UsernameKey: "u", PasswordKey: "p",
						RefreshInterval: -1 * time.Second,
					},
				},
			},
			err: errAWSSecretNegativeInterval,
		},
		{
			name: "server_aws_missing_arn",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					AWSSecret: &AWSSecretHtpasswdConfig{
						Region: "us-east-1",
					},
				},
			},
			err: errAWSSecretMissingARN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, tt.cfg.Validate(), tt.err)
		})
	}
}
