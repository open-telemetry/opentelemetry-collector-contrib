// Copyright The OpenTelemetry Authors
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

package intracesampler

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	processors, err := cm.Sub("processors")
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	componentID := component.NewIDWithName(typeStr, "")
	sub, err := processors.Sub(componentID.String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.NoError(t, component.ValidateConfig(cfg))
	expectedConfig := &Config{
		SamplingPercentage: 15.3,
		HashSeed:           22,
		ScopeLeaves:        []string{"foo", "bar"},
	}
	assert.Equal(t, expectedConfig, cfg)
}

func TestLoadInvalidConfig(t *testing.T) {
	t.Parallel()
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid.yaml"), factories)
	require.ErrorContains(t, err, "negative sampling rate: -15.30")
}
