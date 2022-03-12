// Copyright The OpenTelemetry Authors
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

package k8sobserver

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
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Extensions, 3)

	defaultConfig := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.EqualValues(t, factory.CreateDefaultConfig(), defaultConfig)

	ownNodeOnly := cfg.Extensions[config.NewComponentIDWithName(typeStr, "own-node-only")]
	assert.EqualValues(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "own-node-only")),
			Node:              "node-1",
			APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
			ObservePods:       true,
		},
		ownNodeOnly)

	observeAll := cfg.Extensions[config.NewComponentIDWithName(typeStr, "observe-all")]
	assert.EqualValues(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "observe-all")),
			Node:              "",
			APIConfig:         k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone},
			ObservePods:       true,
			ObserveNodes:      true,
		},
		observeAll)

}

func TestInvalidAuth(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata/invalid_auth.yaml"), factories)
	require.NotNil(t, cfg)
	require.EqualError(t, err, `extension "k8s_observer" has invalid configuration: invalid authType for kubernetes: not a real auth type`)
}

func TestInvalidNoObserving(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata/invalid_no_observing.yaml"), factories)
	require.NotNil(t, cfg)
	require.EqualError(t, err, `extension "k8s_observer" has invalid configuration: one of observe_pods and observe_nodes must be true`)
}
