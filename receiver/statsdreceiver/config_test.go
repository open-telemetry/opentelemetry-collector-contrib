// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "receiver_settings"),
			expected: &Config{
				NetAddr: confignet.AddrConfig{
					Endpoint:  "localhost:12345",
					Transport: confignet.TransportTypeUDP6,
				},
				SocketPermissions:   0o622,
				AggregationInterval: 70 * time.Second,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{
						StatsdType:   "histogram",
						ObserverType: "gauge",
					},
					{
						StatsdType:   "timing",
						ObserverType: "histogram",
						Histogram: protocol.HistogramConfig{
							MaxSize: 170,
						},
					},
					{
						StatsdType:   "distribution",
						ObserverType: "summary",
						Summary: protocol.SummaryConfig{
							Percentiles: []float64{0, 10, 50, 90, 95, 100},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	type test struct {
		name        string
		cfg         *Config
		expectedErr string
	}

	const (
		negativeAggregationIntervalErr = "aggregation_interval must be a positive duration"
		noObjectNameErr                = "must specify object id for all TimerHistogramMappings"
		statsdTypeNotSupportErr        = "statsd_type is not a supported mapping for histogram and timing metrics: %s"
		observerTypeNotSupportErr      = "observer_type is not supported for histogram and timing metrics: %s"
		invalidHistogramErr            = "histogram configuration requires observer_type: histogram"
		invalidSummaryErr              = "summary configuration requires observer_type: summary"
	)

	tests := []test{
		{
			name: "negativeAggregationInterval",
			cfg: &Config{
				AggregationInterval: -1,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{StatsdType: "timing", ObserverType: "gauge"},
				},
			},
			expectedErr: negativeAggregationIntervalErr,
		},
		{
			name: "emptyStatsdType",
			cfg: &Config{
				AggregationInterval: 10,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{ObserverType: "gauge"},
				},
			},
			expectedErr: noObjectNameErr,
		},
		{
			name: "emptyObserverType",
			cfg: &Config{
				AggregationInterval: 10,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{StatsdType: "timing"},
				},
			},
			expectedErr: noObjectNameErr,
		},
		{
			name: "StatsdTypeNotSupport",
			cfg: &Config{
				AggregationInterval: 10,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{StatsdType: "abc", ObserverType: "gauge"},
				},
			},
			expectedErr: fmt.Sprintf(statsdTypeNotSupportErr, "abc"),
		},
		{
			name: "ObserverTypeNotSupport",
			cfg: &Config{
				AggregationInterval: 10,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{StatsdType: "timer", ObserverType: "gauge1"},
				},
			},
			expectedErr: fmt.Sprintf(observerTypeNotSupportErr, "gauge1"),
		},
		{
			name: "invalidHistogram",
			cfg: &Config{
				AggregationInterval: 20 * time.Second,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{
						StatsdType:   "timing",
						ObserverType: "gauge",
						Histogram: protocol.HistogramConfig{
							MaxSize: 100,
						},
					},
				},
			},
			expectedErr: invalidHistogramErr,
		},
		{
			name: "invalidSummary",
			cfg: &Config{
				AggregationInterval: 20 * time.Second,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{
						StatsdType:   "timing",
						ObserverType: "gauge",
						Summary: protocol.SummaryConfig{
							Percentiles: []float64{1},
						},
					},
				},
			},
			expectedErr: invalidSummaryErr,
		},
		{
			name: "negativeAggregationInterval",
			cfg: &Config{
				AggregationInterval: -1,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{StatsdType: "timing", ObserverType: "gauge"},
				},
			},
			expectedErr: negativeAggregationIntervalErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.cfg.Validate(), test.expectedErr)
		})
	}
}

func TestConfig_Validate_MaxSize(t *testing.T) {
	for _, maxSize := range []int32{structure.MaximumMaxSize + 1, -1, -structure.MaximumMaxSize} {
		cfg := &Config{
			AggregationInterval: 20 * time.Second,
			TimerHistogramMapping: []protocol.TimerHistogramMapping{
				{
					StatsdType:   "timing",
					ObserverType: "histogram",
					Histogram: protocol.HistogramConfig{
						MaxSize: maxSize,
					},
				},
			},
		}
		err := cfg.Validate()
		assert.ErrorContains(t, err, "histogram max_size out of range")
	}
}

func TestConfig_Validate_HistogramGoodConfig(t *testing.T) {
	for _, maxSize := range []int32{structure.MaximumMaxSize, 0, 2} {
		cfg := &Config{
			AggregationInterval: 20 * time.Second,
			TimerHistogramMapping: []protocol.TimerHistogramMapping{
				{
					StatsdType:   "timing",
					ObserverType: "histogram",
					Histogram: protocol.HistogramConfig{
						MaxSize: maxSize,
					},
				},
			},
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	}
}
