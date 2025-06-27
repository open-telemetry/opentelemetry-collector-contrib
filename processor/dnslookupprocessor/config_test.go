// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		errMsg   string
	}{
		{
			id:     component.NewIDWithName(metadata.Type, "invalid_attributes"),
			errMsg: "resolve configuration: at least one source_attributes must be specified for DNS resolution",
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom_attributes"),
			expected: &Config{
				Resolve: LookupConfig{
					Enabled:          true,
					Context:          resource,
					SourceAttributes: []string{"custom.address", "proxy.address"},
					TargetAttribute:  "custom.ip",
				},
				Reverse: LookupConfig{
					Enabled:          true,
					Context:          resource,
					SourceAttributes: []string{"custom.address", "proxy.address"},
					TargetAttribute:  "custom.ip",
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

			if tt.errMsg != "" {
				assert.EqualError(t, xconfmap.Validate(cfg), tt.errMsg)
				return
			}

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	createValidConfig := func() Config {
		return Config{
			Resolve: LookupConfig{
				Enabled:          true,
				Context:          resource,
				SourceAttributes: []string{"host.name"},
				TargetAttribute:  "host.ip",
			},
			Reverse: LookupConfig{
				Enabled:          false,
				Context:          resource,
				SourceAttributes: []string{"client.ip"},
				TargetAttribute:  "client.name",
			},
		}
	}

	tests := []struct {
		name             string
		mutateConfigFunc func(*Config)
		expectError      bool
		errorMsg         string
	}{
		{
			name:             "Valid default configuration",
			mutateConfigFunc: func(_ *Config) {},
			expectError:      false,
		},
		{
			name: "Empty resolve attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.SourceAttributes = []string{}
			},
			expectError: true,
			errorMsg:    "resolve configuration: at least one source_attributes must be specified for DNS resolution",
		},
		{
			name: "Empty reverse attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.SourceAttributes = []string{}
			},
			expectError: true,
			errorMsg:    "reverse configuration: at least one source_attributes must be specified for DNS resolution",
		},
		{
			name: "Missing resolve target_attribute",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.TargetAttribute = ""
			},
			expectError: true,
			errorMsg:    "resolve configuration: target_attribute must be specified for DNS resolution",
		},
		{
			name: "Missing reverse target_attribute",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.TargetAttribute = ""
			},
			expectError: true,
			errorMsg:    "reverse configuration: target_attribute must be specified for DNS resolution",
		},
		{
			name: "Invalid resolve context",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Context = "invalid"
			},
			expectError: true,
			errorMsg:    "resolve configuration: context must be either 'resource' or 'record'",
		},
		{
			name: "Invalid reverse context",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.Enabled = false
				cfg.Reverse.Enabled = true
				cfg.Reverse.Context = "invalid"
			},
			expectError: true,
			errorMsg:    "reverse configuration: context must be either 'resource' or 'record'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createValidConfig()
			tt.mutateConfigFunc(&cfg)

			err := cfg.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
