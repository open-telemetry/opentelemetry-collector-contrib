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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Receivers, 1)

	receiverConfig := cfg.Receivers[config.NewComponentID(typeStr)]
	defaultConfig := factory.CreateDefaultConfig().(*Config)
	defaultConfig.Host = "localhost:5665"
	defaultConfig.Username = "root"
	defaultConfig.Password = "test"
	defaultConfig.DisableSslVerification = true
	defaultConfig.Filter = "foo_bar"
	defaultConfig.Histograms = []HistogramConfig{{
		Service: "stp_cpu-usage",
		Host:    ".*",
		Type:    "cpu_system",
		Values:  []float64{.1, .5, .9},
	}}
	assert.Equal(t, defaultConfig, receiverConfig)
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalid.yaml"), factories)

	require.EqualError(t, err, "receiver \"icinga\" has invalid configuration: histogram with service=\"stp_cpu-usage\", host=\".*\", type=\"cpu_system\" does not have strictly increasing values")
}

func TestInvalidConfigValidation(t *testing.T) {
	configuration := loadSuccessfulConfig(t)
	configuration.Host = ""
	configuration.Username = ""
	require.EqualError(t, configuration.Validate(), "host not specified; username not specified")
}

func loadSuccessfulConfig(t *testing.T) *Config {
	configuration := &Config{
		Host:                   "localhost:5665",
		Username:               "root",
		Password:               "test",
		DisableSslVerification: false,
		Histograms:             []HistogramConfig{},
	}

	require.NoError(t, configuration.Validate())
	return configuration
}
