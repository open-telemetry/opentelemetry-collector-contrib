// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerprovider

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerprovider/internal/metadata"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
		err  error
	}{
		{
			name: "valid",
			cfg: &Config{
				SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
				Region:          "us-east-1",
				RefreshInterval: 60 * time.Minute,
			},
		},
		{
			name: "missing_secret_arn",
			cfg: &Config{
				Region:          "us-east-1",
				RefreshInterval: 60 * time.Minute,
			},
			err: errMissingSecretARN,
		},
		{
			name: "missing_region",
			cfg: &Config{
				SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
				RefreshInterval: 60 * time.Minute,
			},
			err: errMissingRegion,
		},
		{
			name: "negative_refresh_interval",
			cfg: &Config{
				SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
				Region:          "us-east-1",
				RefreshInterval: -1 * time.Second,
			},
			err: errNegativeRefreshInterval,
		},
		{
			name: "zero_refresh_interval",
			cfg: &Config{
				SecretARN: "arn:aws:secretsmanager:us-east-1:123:secret:test",
				Region:    "us-east-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(metadata.Type.String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.NoError(t, xconfmap.Validate(cfg))

	expected := &Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
		Region:          "us-east-1",
		RefreshInterval: 60 * time.Minute,
	}
	assert.Equal(t, expected, cfg)
}
