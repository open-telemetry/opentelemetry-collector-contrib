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

package k8sclusterreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	receiverType := "k8s_cluster"
	factories.Receivers[config.Type(receiverType)] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r1 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "all_settings")].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings:           config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "all_settings")),
			Distribution:               distributionKubernetes,
			CollectionInterval:         30 * time.Second,
			NodeConditionTypesToReport: []string{"Ready", "MemoryPressure"},
			MetadataExporters:          []string{"nop"},
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		})

	r3 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "partial_settings")].(*Config)
	assert.Equal(t, r3,
		&Config{
			ReceiverSettings:           config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "partial_settings")),
			Distribution:               distributionOpenShift,
			CollectionInterval:         30 * time.Second,
			NodeConditionTypesToReport: []string{"Ready"},
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		})
}
