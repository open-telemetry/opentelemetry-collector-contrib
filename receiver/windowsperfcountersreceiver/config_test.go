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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	negativeCollectionIntervalErr = "collection_interval must be a positive duration"
	noPerfCountersErr             = "must specify at least one perf counter"
	noObjectNameErr               = "must specify object name for all perf counters"
	noCountersErr                 = `perf counter for object "%s" does not specify any counters`
	emptyInstanceErr              = `perf counter for object "%s" includes an empty instance`
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	counterConfig := CounterConfig{
		Name: "counter1",
		MetricRep: MetricRep{
			Name: "metric",
		},
	}
	singleObject := createDefaultConfig()
	singleObject.(*Config).PerfCounters = []ObjectConfig{{Object: "object", Counters: []CounterConfig{counterConfig}}}
	singleObject.(*Config).MetricMetaData = map[string]MetricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       GaugeMetric{},
		},
	}

	tests := []struct {
		id          config.ComponentID
		expected    config.Receiver
		expectedErr string
	}{
		{
			id:       config.NewComponentIDWithName(typeStr, ""),
			expected: singleObject,
		},
		{
			id: config.NewComponentIDWithName(typeStr, "customname"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 30 * time.Second,
				},
				PerfCounters: []ObjectConfig{
					{
						Object:   "object1",
						Counters: []CounterConfig{counterConfig},
					},
					{
						Object: "object2",
						Counters: []CounterConfig{
							counterConfig,
							{
								Name: "counter2",
								MetricRep: MetricRep{
									Name: "metric2",
								},
							},
						},
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
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "nometrics"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 60 * time.Second,
				},
				PerfCounters: []ObjectConfig{
					{
						Object:   "object",
						Counters: []CounterConfig{{Name: "counter1"}},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "nometricspecified"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 60 * time.Second,
				},
				PerfCounters: []ObjectConfig{
					{
						Object:   "object",
						Counters: []CounterConfig{{Name: "counter1"}},
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
			id: config.NewComponentIDWithName(typeStr, "summetric"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 60 * time.Second,
				},
				PerfCounters: []ObjectConfig{
					{
						Object:   "object",
						Counters: []CounterConfig{{Name: "counter1", MetricRep: MetricRep{Name: "metric"}}},
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
			id: config.NewComponentIDWithName(typeStr, "unspecifiedmetrictype"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
					CollectionInterval: 60 * time.Second,
				},
				PerfCounters: []ObjectConfig{
					{
						Object:   "object",
						Counters: []CounterConfig{{Name: "counter1", MetricRep: MetricRep{Name: "metric"}}},
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
			id:          config.NewComponentIDWithName(typeStr, "negative-collection-interval"),
			expectedErr: negativeCollectionIntervalErr,
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "noperfcounters"),
			expectedErr: noPerfCountersErr,
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "noobjectname"),
			expectedErr: noObjectNameErr,
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "nocounters"),
			expectedErr: fmt.Sprintf(noCountersErr, "object"),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "allerrors"),
			expectedErr: fmt.Sprintf(
				"%s; %s; %s; %s",
				negativeCollectionIntervalErr,
				fmt.Sprintf(noCountersErr, "object"),
				fmt.Sprintf(emptyInstanceErr, "object"),
				noObjectNameErr,
			),
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "emptyinstance"),
			expectedErr: fmt.Sprintf(emptyInstanceErr, "object"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			if tt.expectedErr != "" {
				assert.Equal(t, cfg.Validate().Error(), tt.expectedErr)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
