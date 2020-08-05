// Copyright 2019, OpenTelemetry Authors
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

package splunkhecreceiver

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

type mockMetricsConsumer struct {
}

var _ consumer.MetricsConsumer = (*mockMetricsConsumer)(nil)

func (m *mockMetricsConsumer) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return nil
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	tReceiver, err := createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, &mockMetricsConsumer{})
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := createTraceReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	tReceiver, err := createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, &mockMetricsConsumer{})
	assert.Error(t, err, "endpoint is not formatted correctly: missing port in address")
	assert.Nil(t, tReceiver)
}

func TestCreateNoPort(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:"

	tReceiver, err := createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, &mockMetricsConsumer{})
	assert.Error(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"\": invalid syntax")
	assert.Nil(t, tReceiver)
}

func TestCreateLargePort(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:65536"

	tReceiver, err := createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, &mockMetricsConsumer{})
	assert.Error(t, err, "port number must be between 1 and 65535")
	assert.Nil(t, tReceiver)
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, configmodels.Type("splunk_hec"), NewFactory().Type())
}
