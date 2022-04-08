// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windowsperfcountersreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/service/servicetest"

	windowsapi "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)]
	defaultConfigSingleObject := factory.CreateDefaultConfig()

	counterConfig := windowsapi.CounterConfig{
		Name: "counter1",
		MetricRep: windowsapi.MetricRep{
			Name: "metric",
		},
	}
	defaultConfigSingleObject.(*Config).PerfCounters = []windowsapi.ObjectConfig{{Object: "object", Counters: []windowsapi.CounterConfig{counterConfig}}}
	defaultConfigSingleObject.(*Config).MetricMetaData = map[string]MetricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       GaugeMetric{},
		},
	}

	assert.Equal(t, defaultConfigSingleObject, r0)

	counterConfig2 := windowsapi.CounterConfig{
		Name: "counter2",
		MetricRep: windowsapi.MetricRep{

			Name: "metric2",
		},
	}

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "customname")].(*Config)
	expectedConfig := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "customname")),
			CollectionInterval: 30 * time.Second,
		},
		PerfCounters: []windowsapi.ObjectConfig{
			{
				Object:   "object1",
				Counters: []windowsapi.CounterConfig{counterConfig},
			},
			{
				Object:   "object2",
				Counters: []windowsapi.CounterConfig{counterConfig, counterConfig2},
			},
		},
		MetricMetaData: map[string]MetricConfig{
			"metric": {
				Description: "desc",
				Unit:        "1",
				Gauge:       GaugeMetric{},
			},
			"metric2": {
				Description: "desc",
				Unit:        "1",
				Gauge:       GaugeMetric{},
			},
		},
	}

	assert.Equal(t, expectedConfig, r1)
}

func TestLoadConfigMetrics(t *testing.T) {
	testCases := []struct {
		TestName string
		TestPath string
		Expected Config
	}{
		{
			TestName: "NoMetricsDefined",
			TestPath: filepath.Join("testdata", "config-nometrics.yaml"),
			Expected: Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{
						Object:   "object",
						Counters: []windowsapi.CounterConfig{{Name: "counter1"}},
					},
				},
			},
		},
		{
			TestName: "NoMetricSpecified",
			TestPath: filepath.Join("testdata", "config-nometricspecified.yaml"),
			Expected: Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{
						Object:   "object",
						Counters: []windowsapi.CounterConfig{{Name: "counter1"}},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric": {
						Description: "desc",
						Unit:        "1",
						Gauge:       GaugeMetric{},
					},
				},
			},
		},
		{
			TestName: "SumMetric",
			TestPath: filepath.Join("testdata", "config-summetric.yaml"),
			Expected: Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{
						Object:   "object",
						Counters: []windowsapi.CounterConfig{{Name: "counter1", MetricRep: windowsapi.MetricRep{Name: "metric"}}},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric": {
						Description: "desc",
						Unit:        "1",
						Sum: SumMetric{
							Aggregation: "cumulative",
							Monotonic:   false,
						},
					},
				},
			},
		},
		{
			TestName: "MetricUnspecifiedType",
			TestPath: filepath.Join("testdata", "config-unspecifiedmetrictype.yaml"),
			Expected: Config{
				PerfCounters: []windowsapi.ObjectConfig{
					{
						Object:   "object",
						Counters: []windowsapi.CounterConfig{{Name: "counter1", MetricRep: windowsapi.MetricRep{Name: "metric"}}},
					},
				},
				MetricMetaData: map[string]MetricConfig{
					"metric": {
						Description: "desc",
						Unit:        "1",
						Gauge:       GaugeMetric{},
					},
				},
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			require.NoError(t, err)

			factory := NewFactory()
			factories.Receivers[typeStr] = factory
			cfg, err := servicetest.LoadConfigAndValidate(test.TestPath, factories)

			require.NoError(t, err)
			require.NotNil(t, cfg)

			assert.Equal(t, len(cfg.Receivers), 1)

			actualReceiver := cfg.Receivers[config.NewComponentID(typeStr)]
			expectedReceiver := factory.CreateDefaultConfig()
			expectedReceiver.(*Config).PerfCounters = test.Expected.PerfCounters
			expectedReceiver.(*Config).MetricMetaData = test.Expected.MetricMetaData

			assert.Equal(t, expectedReceiver, actualReceiver)
		})
	}
}

func TestLoadConfig_Error(t *testing.T) {
	type testCase struct {
		name        string
		cfgFile     string
		expectedErr string
	}

	const (
		errorPrefix                   = "receiver \"windowsperfcounters\" has invalid configuration"
		negativeCollectionIntervalErr = "collection_interval must be a positive duration"
		noPerfCountersErr             = "must specify at least one perf counter"
		noObjectNameErr               = "must specify object name for all perf counters"
		noCountersErr                 = `perf counter for object "%s" does not specify any counters`
		emptyInstanceErr              = `perf counter for object "%s" includes an empty instance`
	)

	testCases := []testCase{
		{
			name:        "NegativeCollectionInterval",
			cfgFile:     "config-negative-collection-interval.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, negativeCollectionIntervalErr),
		},
		{
			name:        "NoPerfCounters",
			cfgFile:     "config-noperfcounters.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, noPerfCountersErr),
		},
		{
			name:        "NoObjectName",
			cfgFile:     "config-noobjectname.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, noObjectNameErr),
		},
		{
			name:        "NoCounters",
			cfgFile:     "config-nocounters.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, fmt.Sprintf(noCountersErr, "object")),
		},
		{
			name:        "EmptyInstance",
			cfgFile:     "config-emptyinstance.yaml",
			expectedErr: fmt.Sprintf("%s: %s", errorPrefix, fmt.Sprintf(emptyInstanceErr, "object")),
		},
		{
			name:    "AllErrors",
			cfgFile: "config-allerrors.yaml",
			expectedErr: fmt.Sprintf(
				"%s: %s; %s; %s; %s",
				errorPrefix,
				negativeCollectionIntervalErr,
				fmt.Sprintf(noCountersErr, "object"),
				fmt.Sprintf(emptyInstanceErr, "object"),
				noObjectNameErr,
			),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			require.NoError(t, err)

			factory := NewFactory()
			factories.Receivers[typeStr] = factory
			_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", test.cfgFile), factories)

			require.EqualError(t, err, test.expectedErr)
		})
	}
}
