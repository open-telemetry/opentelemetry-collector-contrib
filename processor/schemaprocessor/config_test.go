// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, &Config{
		HTTPClientSettings: confighttp.NewDefaultHTTPClientSettings(),
		Prefetch: []string{
			"https://opentelemetry.io/schemas/1.9.0",
		},
		Targets: []string{
			"https://opentelemetry.io/schemas/1.4.2",
			"https://example.com/otel/schemas/1.2.0",
		},
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

		assert.ErrorIs(t, component.ValidateConfig(cfg), tc.expectError, tc.scenario)
	}
}
