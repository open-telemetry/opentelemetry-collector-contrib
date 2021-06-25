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

package awscontainerinsightreceiver

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

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	//ensure default configurations are generated when users provide nothing
	r0 := cfg.Receivers[config.NewID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Receivers[config.NewID(typeStr)]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers[config.NewIDWithName(typeStr, "collection_interval_settings")].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings:      config.NewReceiverSettings(config.NewIDWithName(typeStr, "collection_interval_settings")),
			CollectionInterval:    60 * time.Second,
			ContainerOrchestrator: "eks",
			TagService:            true,
			PrefFullPodName:       false,
		})
}
