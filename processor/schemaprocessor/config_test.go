// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemaprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(config.NewComponentIDWithName(typeStr, "with-all-options").String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalProcessor(sub, cfg))

	assert.Equal(t, &Config{
		ProcessorSettings:  config.NewProcessorSettings(config.NewComponentID(typeStr)),
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

		assert.ErrorIs(t, cfg.Validate(), tc.expectError, tc.scenario)
	}
}
