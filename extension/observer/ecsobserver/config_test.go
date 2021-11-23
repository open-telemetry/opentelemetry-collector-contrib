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

package ecsobserver

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Extensions, 4)

	// Default
	ext0 := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), ext0)

	// Merge w/ Default
	ext1 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "1")]
	assert.Equal(t, DefaultConfig().ClusterName, ext1.(*Config).ClusterName)
	assert.NotEqual(t, DefaultConfig().ClusterRegion, ext1.(*Config).ClusterRegion)
	assert.Equal(t, "my_prometheus_job", ext1.(*Config).JobLabelName)

	// Example Config
	ext2 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "2")]
	ext2Expected := exampleConfig()
	ext2Expected.ExtensionSettings = config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "2"))
	assert.Equal(t, &ext2Expected, ext2)

	// Override docker label from default
	ext3 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "3")]
	ext3Expected := DefaultConfig()
	ext3Expected.ExtensionSettings = config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "3"))
	ext3Expected.DockerLabels = []DockerLabelConfig{
		{
			PortLabel: "IS_NOT_DEFAULT",
		},
	}
	assert.Equal(t, &ext3Expected, ext3)
}

func TestConfig_Validate(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		factories, err := componenttest.NopFactories()
		require.NoError(t, err)

		factory := NewFactory()
		factories.Extensions[typeStr] = factory
		_, err = configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config_invalid.yaml"), factories)
		require.Error(t, err)
	})

	cases := []struct {
		reason string
		cfg    Config
	}{
		{
			reason: "cluster name",
			cfg:    Config{ClusterName: ""},
		},
		{
			reason: "service",
			cfg: Config{
				ClusterName: "c1",
				Services:    []ServiceConfig{{NamePattern: "*"}}, // invalid regex
			},
		},
		{
			reason: "task",
			cfg: Config{
				ClusterName:     "c1",
				TaskDefinitions: []TaskDefinitionConfig{{ArnPattern: "*"}}, // invalid regex
			},
		},
		{
			reason: "docker",
			cfg: Config{
				ClusterName:  "c1",
				DockerLabels: []DockerLabelConfig{{PortLabel: ""}},
			},
		},
	}

	for _, tCase := range cases {
		t.Run(tCase.reason, func(t *testing.T) {
			require.Error(t, tCase.cfg.Validate())
		})
	}
}
