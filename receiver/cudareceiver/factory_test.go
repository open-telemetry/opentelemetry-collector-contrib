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

package cudareceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

type mockMetricsConsumer struct{}

var _ consumer.MetricsConsumerOld = (*mockMetricsConsumer)(nil)

func (m *mockMetricsConsumer) ConsumeMetricsData(_ context.Context, _ consumerdata.MetricsData) error {
	return nil
}
func TestCreateReceiver(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	// Mock NewCUDAMetricsCollector to run tests on instances without GPU devices.
	var siding func(time.Duration, string, *zap.Logger, consumer.MetricsConsumerOld) (*CUDAMetricsCollector, error)
	siding = NewCUDAMetricsCollector
	defer func() { NewCUDAMetricsCollector = siding }()

	NewCUDAMetricsCollector = func(d time.Duration, prefix string, logger *zap.Logger, con consumer.MetricsConsumerOld) (*CUDAMetricsCollector, error) {
		return &CUDAMetricsCollector{
			consumer:       con,
			startTime:      time.Now(),
			device:         nil,
			scrapeInterval: d,
			metricPrefix:   prefix,
			done:           make(chan struct{}),
			logger:         logger,
		}, nil

	}
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}
