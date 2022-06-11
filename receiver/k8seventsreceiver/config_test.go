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

package k8seventsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	receiverType := "k8s_events"
	factories.Receivers[config.Type(receiverType)] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Receivers), 2)

	r1 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, r1, factory.CreateDefaultConfig())

	r2 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "all_settings")].(*Config)
	assert.Equal(t, r2,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "all_settings")),
			Namespaces:       []string{"default", "my_namespace"},
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeServiceAccount,
			},
		})
}
