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

package statsdreceiver

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Receiver
	}{
		{
			id:       config.NewComponentID(typeStr),
			expected: createDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "receiver_settings"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:12345",
					Transport: "custom_transport",
				},
				AggregationInterval: 70 * time.Second,
				TimerHistogramMapping: []protocol.TimerHistogramMapping{
					{
						StatsdType:   "histogram",
						ObserverType: "gauge",
					},
					{
						StatsdType:   "timing",
						ObserverType: "gauge",
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
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			assert.NoError(t, cfg.Validate())
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
		statsdTypeNotSupportErr        = "statsd_type is not a supported mapping: %s"
		observerTypeNotSupportErr      = "observer_type is not supported: %s"
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualError(t, test.cfg.validate(), test.expectedErr)
		})
	}

}
