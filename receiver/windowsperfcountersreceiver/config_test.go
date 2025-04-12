// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsperfcountersreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/metadata"
)

const (
	negativeCollectionIntervalErr = "\"collection_interval\": requires positive value"
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
		id           component.ID
		expected     component.Config
		expectedErrs []string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: singleObject,
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       time.Second,
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
			id: component.NewIDWithName(metadata.Type, "nometrics"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 60 * time.Second,
					InitialDelay:       time.Second,
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
			id: component.NewIDWithName(metadata.Type, "nometricspecified"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 60 * time.Second,
					InitialDelay:       time.Second,
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
			id: component.NewIDWithName(metadata.Type, "summetric"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 60 * time.Second,
					InitialDelay:       time.Second,
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
			id: component.NewIDWithName(metadata.Type, "unspecifiedmetrictype"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 60 * time.Second,
					InitialDelay:       time.Second,
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
			id:           component.NewIDWithName(metadata.Type, "negative-collection-interval"),
			expectedErrs: []string{"collection_interval must be a positive duration", negativeCollectionIntervalErr},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "noperfcounters"),
			expectedErrs: []string{noPerfCountersErr},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "noobjectname"),
			expectedErrs: []string{noObjectNameErr},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "nocounters"),
			expectedErrs: []string{fmt.Sprintf(noCountersErr, "object")},
		},
		{
			id: component.NewIDWithName(metadata.Type, "allerrors"),
			expectedErrs: []string{
				"collection_interval must be a positive duration",
				fmt.Sprintf(noCountersErr, "object"),
				fmt.Sprintf(emptyInstanceErr, "object"),
				noObjectNameErr,
				negativeCollectionIntervalErr,
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "emptyinstance"),
			expectedErrs: []string{fmt.Sprintf(emptyInstanceErr, "object")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if len(tt.expectedErrs) > 0 {
				for _, err := range tt.expectedErrs {
					assert.ErrorContains(t, xconfmap.Validate(cfg), err)
				}
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
