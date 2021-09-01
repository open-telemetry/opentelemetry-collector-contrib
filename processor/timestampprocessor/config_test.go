// Copyright  OpenTelemetry Authors
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

package timestampprocessor

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

// TestLoadingConfigRegexp tests loading testdata/config_strict.yaml
func TestLoadingConfigStrict(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	assert.Nil(t, err)
	require.NotNil(t, cfg)

	oneSecond := time.Second

	tests := []struct {
		filterID config.ComponentID
		expCfg   *Config
	}{
		{
			filterID: config.NewIDWithName("timestamp", "1sec"),
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewIDWithName(typeStr, "1sec")),
				RoundToNearest:    &oneSecond,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.filterID.String(), func(t *testing.T) {
			cfg := cfg.Processors[test.filterID]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
