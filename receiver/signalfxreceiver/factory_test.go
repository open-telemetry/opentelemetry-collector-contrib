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

package signalfxreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateReceiverMetricsFirst(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	_, err = factory.CreateTraceReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)

	lReceiver, err := factory.CreateLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, consumertest.NewLogsNop())
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	assert.Same(t, mReceiver, lReceiver)
}

func TestCreateReceiverLogsFirst(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	lReceiver, err := factory.CreateLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, consumertest.NewLogsNop())
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	assert.Same(t, mReceiver, lReceiver)
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = ""

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.Error(t, err, "endpoint is not formatted correctly: missing port in address")
	assert.Nil(t, tReceiver)

	tReceiver, err = factory.CreateLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, consumertest.NewLogsNop())
	assert.Error(t, err, "endpoint is not formatted correctly: missing port in address")
	assert.Nil(t, tReceiver)
}

func TestCreateNoPort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:"

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.Error(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"\": invalid syntax")
	assert.Nil(t, tReceiver)
}

func TestCreateLargePort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:65536"

	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.Error(t, err, "port number must be between 1 and 65535")
	assert.Nil(t, tReceiver)
}
