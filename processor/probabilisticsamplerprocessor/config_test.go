// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probabilisticsamplerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, p0,
		&Config{
			ProcessorSettings:  config.NewProcessorSettings(config.NewComponentID(typeStr)),
			SamplingPercentage: 15.3,
			HashSeed:           22,
		})
	p1 := cfg.Processors[config.NewComponentIDWithName(typeStr, "logs")]
	assert.Equal(t,
		&Config{
			ProcessorSettings:  config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "logs")),
			SamplingPercentage: 15.3,
			HashSeed:           22,
			Severity: []severityPair{
				{Level: "error", SamplingPercentage: 100},
				{Level: "warn", SamplingPercentage: 80},
			},
		}, p1)
}

func TestLoadConfigEmpty(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "empty.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, p0, createDefaultConfig())
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid.yaml"), factories)
	require.ErrorContains(t, err, "severity already used: error")
}

func TestNegativeSamplingRate(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.(*Config).SamplingPercentage = -5
	err := cfg.Validate()
	require.ErrorContains(t, err, "negative sampling rate: -5.00")

	cfg = createDefaultConfig()
	cfg.(*Config).Severity = []severityPair{
		{Level: "error", SamplingPercentage: -4.344},
	}
	err = cfg.Validate()
	require.ErrorContains(t, err, "negative sampling rate: -4.34 [error]")
}
