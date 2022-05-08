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

package pulsarreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Topic:            defaultTopic,
		Encoding:         defaultEncoding,
		ConsumerName:     defaultConsumerName,
		Subscription:     defaultSubscription,
		ServiceUrl:       defaultServiceUrl,
	}, cfg)
}

//trace
func TestCreateTracesReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = "invalid:6650"

	f := PulsarReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	r, err := f.createTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateTracesReceiver_err_auth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	r, err := f.createTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateTracesReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{tracesUnmarshalers: make(map[string]TracesUnmarshaler)}
	r, err := f.createTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateTraceReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	f := PulsarReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	recv, err := f.createTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

//metrics
func TestCreateMetricsReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = "invalid:6650"

	f := PulsarReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}
	r, err := f.createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsReceiver_err_auth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}
	r, err := f.createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{metricsUnmarshalers: make(map[string]MetricsUnmarshaler)}
	r, err := f.createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	f := PulsarReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}

	recv, err := f.createMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

//logs
func TestCreateLogsReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = "invalid:6650"

	f := PulsarReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateLogsReceiver_err_auth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateLogsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{logsUnmarshalers: make(map[string]LogsUnmarshaler)}
	r, err := f.createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ServiceUrl = defaultServiceUrl

	f := PulsarReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	recv, err := f.createLogsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}
