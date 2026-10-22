// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrypolicyprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/telemetrypolicyprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewID(metadata.Type).String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, &Config{
		Providers: []component.ID{
			component.MustNewID("file_telemetry_policy"),
		},
	}, cfg)

	subCustom, err := cm.Sub(component.MustNewIDWithName("telemetry_policy", "custom").String())
	require.NoError(t, err)
	cfgCustom := factory.CreateDefaultConfig()
	require.NoError(t, subCustom.Unmarshal(cfgCustom))

	assert.Equal(t, &Config{
		Providers: []component.ID{
			component.MustNewID("file_telemetry_policy"),
			component.MustNewIDWithName("custom_policy_provider", "prod"),
		},
	}, cfgCustom)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "valid config",
			cfg: &Config{
				Providers: []component.ID{
					component.MustNewID("file_telemetry_policy"),
				},
			},
		},
		{
			name: "valid config with multiple providers",
			cfg: &Config{
				Providers: []component.ID{
					component.MustNewID("file_telemetry_policy"),
					component.MustNewIDWithName("custom_policy_provider", "prod"),
				},
			},
		},
		{
			name: "empty providers",
			cfg: &Config{
				Providers: []component.ID{},
			},
			expectedErr: "at least one provider must be specified",
		},
		{
			name: "nil providers",
			cfg: &Config{
				Providers: nil,
			},
			expectedErr: "at least one provider must be specified",
		},
		{
			name: "empty provider ID",
			cfg: &Config{
				Providers: []component.ID{
					{},
				},
			},
			expectedErr: "provider ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}
