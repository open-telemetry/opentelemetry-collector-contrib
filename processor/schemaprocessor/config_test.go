// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "with-all-options").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, &Config{
		ClientConfig:    confighttp.NewDefaultClientConfig(),
		CacheCooldown:   10 * time.Minute,
		CacheRetryLimit: 3,
		Prefetch: []string{
			"https://opentelemetry.io/schemas/1.9.0",
		},
		Targets: []string{
			"https://opentelemetry.io/schemas/1.4.2",
			"https://example.com/otel/schemas/1.2.0",
		},
		Migration: []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.4.2", From: "https://opentelemetry.io/schemas/1.0.0"}},
	}, cfg)
}

func TestConfigurationValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario    string
		target      []string
		expectError error
	}{
		{scenario: "No targets", target: nil, expectError: errRequiresTargets},
		{
			scenario:    "One target of incomplete schema family",
			target:      []string{"opentelemetry.io/schemas/1.0.0"},
			expectError: translation.ErrInvalidFamily,
		},
		{
			scenario:    "One target of incomplete schema identifier",
			target:      []string{"https://opentelemetry.io/schemas/1"},
			expectError: translation.ErrInvalidVersion,
		},
		{
			scenario: "Valid target(s)",
			target: []string{
				"https://opentelemetry.io/schemas/1.9.0",
			},
			expectError: nil,
		},
		{
			scenario: "Duplicate targets",
			target: []string{
				"https://opentelemetry.io/schemas/1.9.0",
				"https://opentelemetry.io/schemas/1.0.0",
			},
			expectError: errDuplicateTargets,
		},
	}

	for _, tc := range tests {
		cfg := &Config{
			Targets: tc.target,
		}

		assert.ErrorIs(t, xconfmap.Validate(cfg), tc.expectError, tc.scenario)
	}
}

func TestMigrationConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario    string
		migration   []MigrationEntry
		expectError error
	}{
		{
			scenario:  "nil migration config",
			migration: nil,
		},
		{
			scenario:    "empty from",
			migration:   []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.9.0"}},
			expectError: errMigrationRequiresFrom,
		},
		{
			scenario:  "valid from matching target family",
			migration: []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.9.0", From: "https://opentelemetry.io/schemas/1.8.0"}},
		},
		{
			scenario:    "invalid from URL",
			migration:   []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.9.0", From: "not-a-valid-url"}},
			expectError: translation.ErrInvalidVersion,
		},
		{
			scenario:    "from and target family mismatch",
			migration:   []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.9.0", From: "https://example.com/schemas/1.0.0"}},
			expectError: errMigrationFamilyMismatch,
		},
		{
			scenario:    "target not in configured targets",
			migration:   []MigrationEntry{{Target: "https://opentelemetry.io/schemas/1.8.0", From: "https://opentelemetry.io/schemas/1.7.0"}},
			expectError: errMigrationTargetNotFound,
		},
		{
			scenario: "duplicate migration target",
			migration: []MigrationEntry{
				{Target: "https://opentelemetry.io/schemas/1.9.0", From: "https://opentelemetry.io/schemas/1.8.0"},
				{Target: "https://opentelemetry.io/schemas/1.9.0", From: "https://opentelemetry.io/schemas/1.7.0"},
			},
			expectError: errMigrationDuplicateTarget,
		},
	}

	for _, tc := range tests {
		cfg := &Config{
			Targets:   []string{"https://opentelemetry.io/schemas/1.9.0"},
			Migration: tc.migration,
		}

		assert.ErrorIs(t, xconfmap.Validate(cfg), tc.expectError, tc.scenario)
	}
}

func TestConfigurationValidation_CacheFields(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Targets:       []string{"https://opentelemetry.io/schemas/1.9.0"},
		CacheCooldown: -1 * time.Minute,
	}
	err := xconfmap.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache_cooldown must not be negative")

	cfg = &Config{
		Targets:         []string{"https://opentelemetry.io/schemas/1.9.0"},
		CacheRetryLimit: -1,
	}
	err = xconfmap.Validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache_retry_limit must not be negative")
}
