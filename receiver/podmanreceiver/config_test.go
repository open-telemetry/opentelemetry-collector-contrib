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

package podmanreceiver

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
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 2, len(cfg.Receivers))

	defaultConfig := cfg.Receivers[config.NewID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), defaultConfig)

	dcfg := defaultConfig.(*Config)
	assert.Equal(t, "podman_stats", dcfg.ID().String())
	assert.Equal(t, "unix:///run/podman/podman.sock", dcfg.Endpoint)
	assert.Equal(t, 10*time.Second, dcfg.CollectionInterval)

	ascfg := cfg.Receivers[config.NewIDWithName(typeStr, "all")].(*Config)
	assert.Equal(t, "podman_stats/all", ascfg.ID().String())
	assert.Equal(t, "http://example.com/", ascfg.Endpoint)
	assert.Equal(t, 2*time.Second, ascfg.CollectionInterval)
}
