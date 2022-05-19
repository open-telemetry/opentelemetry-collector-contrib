// Copyright  The OpenTelemetry Authors
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

package expvarreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 2, len(cfg.Receivers))

	// Validate default config
	expectedCfg := factory.CreateDefaultConfig().(*Config)
	expectedCfg.SetIDName("default")

	assert.Equal(t, expectedCfg, cfg.Receivers[config.NewComponentIDWithName(typeStr, "default")])

	// Validate custom config
	expectedCfg = factory.CreateDefaultConfig().(*Config)
	expectedCfg.SetIDName("custom")
	expectedCfg.CollectionInterval = time.Second * 30
	expectedCfg.HTTP = &confighttp.HTTPClientSettings{
		Endpoint: "http://localhost:8000/custom/path",
		Timeout:  time.Second * 5,
	}
	expectedCfg.MetricsConfig = metadata.DefaultMetricsSettings()
	expectedCfg.MetricsConfig.ProcessRuntimeMemstatsTotalAlloc.Enabled = true
	expectedCfg.MetricsConfig.ProcessRuntimeMemstatsMallocs.Enabled = false

	assert.Equal(t, expectedCfg, cfg.Receivers[config.NewComponentIDWithName(typeStr, "custom")])
}

func TestFailedLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "bad_schemeless_endpoint_config.yaml"), factories)
	assert.EqualError(t, err, "receiver \"expvar\" has invalid configuration: scheme must be 'http' or 'https', but was 'localhost'")

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "bad_hostless_endpoint_config.yaml"), factories)
	assert.EqualError(t, err, "receiver \"expvar\" has invalid configuration: host not found in HTTP endpoint")

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "bad_collection_interval_config.yaml"), factories)
	assert.EqualError(t, err, "error reading receivers configuration for \"expvar\": 1 error(s) decoding:\n\n* error decoding 'collection_interval': time: invalid duration \"fourminutes\"")

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "bad_metric_config.yaml"), factories)
	assert.EqualError(t, err, "error reading receivers configuration for \"expvar\": 1 error(s) decoding:\n\n* 'metrics.process.runtime.memstats.total_alloc' has invalid keys: invalid_field")

	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "bad_metric_name.yaml"), factories)
	assert.EqualError(t, err, "error reading receivers configuration for \"expvar\": 1 error(s) decoding:\n\n* 'metrics' has invalid keys: bad_metric.name")
}
