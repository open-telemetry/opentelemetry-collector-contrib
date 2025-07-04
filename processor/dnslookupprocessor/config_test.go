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
	semconv "go.opentelemetry.io/otel/semconv/v1.31.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectError bool
		errMsg      string
	}{
		{
			id:          component.NewIDWithName(metadata.Type, "missing_resolve_reverse"),
			expectError: true,
			errMsg:      "at least one of 'resolve' or 'reverse' must be configured",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_context"),
			expectError: true,
			errMsg:      "unknown context invalid_context",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_source_attributes"),
			expectError: true,
			errMsg:      "at least one source_attributes must be specified for DNS resolution",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_empty_target_attribute"),
			expectError: true,
			errMsg:      "target_attribute must be specified for DNS resolution",
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_empty"),
			expected: &Config{
				Resolve: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{string(semconv.SourceAddressKey)},
					TargetAttribute:  sourceIPKey,
				},
				Reverse: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{sourceIPKey},
					TargetAttribute:  string(semconv.SourceAddressKey),
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_no_target_attribute"),
			expected: &Config{
				Resolve: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"custom.address"},
					TargetAttribute:  sourceIPKey,
				},
				Reverse: LookupConfig{},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_no_source_attributes"),
			expected: &Config{
				Resolve: LookupConfig{},
				Reverse: LookupConfig{
					Context:          record,
					SourceAttributes: []string{sourceIPKey},
					TargetAttribute:  "custom.address",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_no_context"),
			expected: &Config{
				Resolve: LookupConfig{},
				Reverse: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"custom.ip"},
					TargetAttribute:  "custom.address",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom_attributes"),
			expected: &Config{
				Resolve: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"custom.address", "proxy.address"},
					TargetAttribute:  "custom.ip",
				},
				Reverse: LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"custom.ip", "proxy.ip"},
					TargetAttribute:  "custom.address",
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

			if errUnmarshal := sub.Unmarshal(cfg); errUnmarshal != nil {
				assert.True(t, tt.expectError, "Expected no error but got: %v", errUnmarshal)
				assert.ErrorContains(t, errUnmarshal, tt.errMsg)
				return
			}

			if errValidate := xconfmap.Validate(cfg); errValidate != nil {
				assert.True(t, tt.expectError, "Expected no error but got: %v", errValidate)
				assert.ErrorContains(t, errValidate, tt.errMsg)
				return
			}

			assert.False(t, tt.expectError, "Expected error but got none")
			assert.Empty(t, tt.errMsg)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	createValidConfig := func() Config {
		return Config{
			Resolve: LookupConfig{
				Context:          resource,
				SourceAttributes: []string{"host.name"},
				TargetAttribute:  "host.ip",
			},
			Reverse: LookupConfig{
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
			name:             "Valid configuration",
			mutateConfigFunc: func(_ *Config) {},
			expectError:      false,
		},
		{
			name: "Empty resolve attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve.SourceAttributes = []string{}
			},
			expectError: true,
			errorMsg:    "invalid resolve configuration: at least one source_attributes must be specified for DNS resolution",
		},
		{
			name: "Empty reverse attribute list",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve = LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"source.address"},
					TargetAttribute:  "source.ip",
				}
				cfg.Reverse = LookupConfig{
					Context:          resource,
					SourceAttributes: []string{},
					TargetAttribute:  "source.address",
				}
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
			errorMsg:    "invalid resolve configuration: target_attribute must be specified for DNS resolution",
		},
		{
			name: "Missing reverse target_attribute",
			mutateConfigFunc: func(cfg *Config) {
				cfg.Resolve = LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"source.address"},
					TargetAttribute:  "source.ip",
				}
				cfg.Reverse = LookupConfig{
					Context:          resource,
					SourceAttributes: []string{"source.ip"},
				}
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
				cfg.Reverse = LookupConfig{
					Context:          "invalid",
					SourceAttributes: []string{"source.ip"},
					TargetAttribute:  "source.address",
				}
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

func TestContextID_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    contextID
		wantErr bool
	}{
		{
			name:    "valid resource lowercase",
			input:   "resource",
			want:    resource,
			wantErr: false,
		},
		{
			name:    "valid record lowercase",
			input:   "record",
			want:    record,
			wantErr: false,
		},
		{
			name:    "valid resource mixed case",
			input:   "Resource",
			want:    resource,
			wantErr: false,
		},
		{
			name:    "valid record mixed case",
			input:   "Record",
			want:    record,
			wantErr: false,
		},
		{
			name:    "invalid empty string",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid unknown value",
			input:   "unknown",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid whitespace",
			input:   " resource ",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c contextID
			err := c.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalText() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("UnmarshalText() unexpected error: %v", err)
				return
			}

			if c != tt.want {
				t.Errorf("UnmarshalText() got %v, want %v", c, tt.want)
			}
		})
	}
}
