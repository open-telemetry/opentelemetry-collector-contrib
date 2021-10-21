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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = "localhost:0" // Endpoint is required, not going to be used here.

	params := componenttest.NewNopReceiverCreateSettings()
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
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.Error(t, err, "aggregation_interval must be a positive duration")
	assert.Nil(t, receiver)

}

func TestCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	receiver, err := createMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		nil,
	)

	assert.Error(t, err, "nil consumer")
	assert.Nil(t, receiver)
}
