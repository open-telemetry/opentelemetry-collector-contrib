// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expected: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
					Resolver: attrs.Resolver{
						IncludeFileName: true, // Default from fileconsumer.NewConfig()
					},
					StartAt:            "end",   // Default from fileconsumer.NewConfig()
					Encoding:           "utf-8", // Default from fileconsumer.NewConfig()
					PollInterval:       200 * time.Millisecond,
					MaxConcurrentFiles: 1024,
					MaxLogSize:         1024 * 1024, // 1MiB
					FingerprintSize:    1000,        // fingerprint.DefaultSize
					InitialBufferSize:  16 * 1024,   // scanner.DefaultBufferSize
					FlushPeriod:        500 * time.Millisecond,
				},
				Encoding: "macosunifiedlogencoding",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "start-at-beginning"),
			expected: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
					Resolver: attrs.Resolver{
						IncludeFileName: true, // Default from fileconsumer.NewConfig()
					},
					StartAt:            "beginning", // Overridden in YAML
					Encoding:           "utf-8",     // Default from fileconsumer.NewConfig()
					PollInterval:       200 * time.Millisecond,
					MaxConcurrentFiles: 1024,
					MaxLogSize:         1024 * 1024, // 1MiB
					FingerprintSize:    1000,        // fingerprint.DefaultSize
					InitialBufferSize:  16 * 1024,   // scanner.DefaultBufferSize
					FlushPeriod:        500 * time.Millisecond,
				},
				Encoding: "macosunifiedlogencoding",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "testdata"),
			expected: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"testdata/test.tracev3"},
					},
					Resolver: attrs.Resolver{
						IncludeFileName: true, // Default from fileconsumer.NewConfig()
					},
					StartAt:            "beginning", // Overridden in YAML
					Encoding:           "utf-8",     // Default from fileconsumer.NewConfig()
					PollInterval:       200 * time.Millisecond,
					MaxConcurrentFiles: 1024,
					MaxLogSize:         1024 * 1024, // 1MiB
					FingerprintSize:    1000,        // fingerprint.DefaultSize
					InitialBufferSize:  16 * 1024,   // scanner.DefaultBufferSize
					FlushPeriod:        500 * time.Millisecond,
				},
				Encoding: "macosunifiedlogencoding",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "no-include"),
			expected: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{}, // No include patterns
					Resolver: attrs.Resolver{
						IncludeFileName: true, // Default from fileconsumer.NewConfig()
					},
					StartAt:            "end",   // Default from fileconsumer.NewConfig()
					Encoding:           "utf-8", // Default from fileconsumer.NewConfig()
					PollInterval:       200 * time.Millisecond,
					MaxConcurrentFiles: 1024,
					MaxLogSize:         1024 * 1024, // 1MiB
					FingerprintSize:    1000,        // fingerprint.DefaultSize
					InitialBufferSize:  16 * 1024,   // scanner.DefaultBufferSize
					FlushPeriod:        500 * time.Millisecond,
				},
				Encoding: "macosunifiedlogencoding",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "empty-include"),
			expected: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{}, // Empty include array
					},
					Resolver: attrs.Resolver{
						IncludeFileName: true, // Default from fileconsumer.NewConfig()
					},
					StartAt:            "end",   // Default from fileconsumer.NewConfig()
					Encoding:           "utf-8", // Default from fileconsumer.NewConfig()
					PollInterval:       200 * time.Millisecond,
					MaxConcurrentFiles: 1024,
					MaxLogSize:         1024 * 1024, // 1MiB
					FingerprintSize:    1000,        // fingerprint.DefaultSize
					InitialBufferSize:  16 * 1024,   // scanner.DefaultBufferSize
					FlushPeriod:        500 * time.Millisecond,
				},
				Encoding: "macosunifiedlogencoding",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			// Don't apply getFileConsumerConfig transformation here - test the raw loaded config
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "macosunifiedlogencoding",
			},
			expectError: false,
		},
		{
			name: "missing encoding",
			config: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "",
			},
			expectError: true,
			errorMsg:    "encoding must be macosunifiedlogencoding for macOS Unified Logging receiver",
		},
		{
			name: "wrong encoding",
			config: &Config{
				BaseConfig: adapter.BaseConfig{
					Operators:      []operator.Config{},
					RetryOnFailure: consumerretry.NewDefaultConfig(),
				},
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "some_other_encoding",
			},
			expectError: true,
			errorMsg:    "encoding must be macosunifiedlogencoding for macOS Unified Logging receiver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFailedLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		name     string
		configID string
		errorMsg string
	}{
		{
			name:     "bad encoding",
			configID: "bad-encoding",
			errorMsg: "encoding must be macosunifiedlogencoding for macOS Unified Logging receiver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, tt.configID).String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = cfg.(*Config).Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestGetFileConsumerConfig(t *testing.T) {
	config := &Config{
		Config: fileconsumer.Config{
			Criteria: matcher.Criteria{
				Include: []string{"/test/*.tracev3"},
			},
			Encoding: "utf-8", // This should be overridden
		},
		Encoding: "macosunifiedlogencoding",
	}

	fcConfig := config.getFileConsumerConfig()

	// Verify that encoding is set to "nop"
	assert.Equal(t, "nop", fcConfig.Encoding)

	// Verify that file attributes are enabled
	assert.True(t, fcConfig.IncludeFilePath)
	assert.True(t, fcConfig.IncludeFileName)

	// Verify that other settings are preserved
	assert.Equal(t, []string{"/test/*.tracev3"}, fcConfig.Include)
}
