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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	receiverType := "k8s_cluster"
	factories.Receivers[configmodels.Type(receiverType)] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	r1 := cfg.Receivers["k8s_cluster"]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers["k8s_cluster/all_settings"].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: configmodels.Type(receiverType),
				NameVal: "k8s_cluster/all_settings",
			},
			CollectionInterval:         30 * time.Second,
			NodeConditionTypesToReport: []string{"Ready", "MemoryPressure"},
			MetadataExporters:          []string{"exampleexporter"},
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		})

	r3 := cfg.Receivers["k8s_cluster/partial_settings"].(*Config)
	assert.Equal(t, r3,
		&Config{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: configmodels.Type(receiverType),
				NameVal: "k8s_cluster/partial_settings",
			},
			CollectionInterval:         30 * time.Second,
			NodeConditionTypesToReport: []string{"Ready"},
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		})
}
