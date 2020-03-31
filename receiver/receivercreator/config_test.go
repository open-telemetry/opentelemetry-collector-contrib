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

package receivercreator

import (
	"path"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exampleCreatorFactory(t *testing.T) *configmodels.Config {
	factories, err := config.ExampleComponents()
	require.Nil(t, err)

	factory := &Factory{}
	factories.Receivers[typeStr] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	return cfg
}

func TestLoadConfig(t *testing.T) {
	cfg := exampleCreatorFactory(t)
	factory := &Factory{}

	r0 := cfg.Receivers["receiver_creator"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Receivers["receiver_creator/1"].(*Config)

	assert.NotNil(t, r1)
	assert.Len(t, r1.subreceiverConfigs, 1)
	assert.Contains(t, r1.subreceiverConfigs, "examplereceiver/1")
	assert.Equal(t, "test rule", r1.subreceiverConfigs["examplereceiver/1"].Rule)
	assert.Equal(t, map[string]interface{}{
		"endpoint": "localhost:12345",
	}, r1.subreceiverConfigs["examplereceiver/1"].config)
}
