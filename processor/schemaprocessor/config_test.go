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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/schema"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err, "Must not error on creating Nop factories")

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config.yml"), factories)
	require.NoError(t, err, "Must not error when loading configuration")

	pcfg := cfg.Processors[config.NewComponentIDWithName(typeStr, "with-all-options")]
	assert.Equal(t, pcfg, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "with-all-options")),
		Prefetch: []string{
			"https://opentelemetry.io/schemas/1.9.0",
		},
		Targets: []string{
			"https://opentelemetry.io/schemas/1.4.2",
			"https://example.com/otel/schemas/1.2.0",
		},
	})
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
			expectError: schema.ErrInvalidFamily,
		},
		{
			scenario:    "One target of incomplete schema identifier",
			target:      []string{"https://opentelemetry.io/schemas/1"},
			expectError: schema.ErrInvalidIdentifier,
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
