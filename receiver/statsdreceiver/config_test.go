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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "receiver_settings")]
	assert.Equal(t, &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "receiver_settings")),
		NetAddr: confignet.NetAddr{
			Endpoint:  "localhost:12345",
			Transport: "custom_transport",
		},
		AggregationInterval:   70 * time.Second,
		TimerHistogramMapping: []protocol.TimerHistogramMapping{{StatsdType: "histogram", ObserverType: "gauge"}, {StatsdType: "timing", ObserverType: "gauge"}},
	}, r1)
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
