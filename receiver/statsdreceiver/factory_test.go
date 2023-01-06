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
	"context"
	"testing"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:0" // Endpoint is required, not going to be used here.

	params := receivertest.NewNopCreateSettings()
	tReceiver, err := createMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "receiver creation failed")
}

func TestCreateReceiverWithConfigErr(t *testing.T) {
	cfg := &Config{
		AggregationInterval: -1,
		TimerHistogramMapping: []protocol.TimerHistogramMapping{
			{StatsdType: "timing", ObserverType: "gauge"},
		},
	}
	receiver, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.Error(t, err, "aggregation_interval must be a positive duration")
	assert.Nil(t, receiver)

}

func TestCreateReceiverWithHistogramConfigError(t *testing.T) {
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
		receiver, err := createMetricsReceiver(
			context.Background(),
			receivertest.NewNopCreateSettings(),
			cfg,
			consumertest.NewNop(),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "histogram max_size out of range")
		assert.Nil(t, receiver)
	}
}

func TestCreateReceiverWithHistogramGoodConfig(t *testing.T) {
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
		receiver, err := createMetricsReceiver(
			context.Background(),
			receivertest.NewNopCreateSettings(),
			cfg,
			consumertest.NewNop(),
		)
		assert.NoError(t, err)
		assert.NotNil(t, receiver)
		assert.NoError(t, receiver.Start(context.Background(), &testHost{t: t}))
		assert.NoError(t, receiver.Shutdown(context.Background()))
	}
}

func TestCreateReceiverWithInvalidHistogramConfig(t *testing.T) {
	cfg := &Config{
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
	}
	receiver, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "histogram configuration requires observer_type: histogram")
	assert.Nil(t, receiver)
}

func TestCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	receiver, err := createMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		createDefaultConfig(),
		nil,
	)

	assert.Error(t, err, "nil consumer")
	assert.Nil(t, receiver)
}
