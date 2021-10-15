// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cumulativetodeltaprocessor

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

const configFile = "config.yaml"

func TestLoadingFullConfig(t *testing.T) {

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", configFile), factories)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	tests := []struct {
		expCfg *Config
	}{
		{
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "alt")),
				Metrics: []string{
					"metric1",
					"metric2",
				},
				MaxStaleness: 10 * time.Second,
			},
		},
		{
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.expCfg.ID().String(), func(t *testing.T) {
			cfg := cfg.Processors[test.expCfg.ID()]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
