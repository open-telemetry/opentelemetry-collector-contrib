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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	mockLogsConsumer := exportertest.NewNopLogsExporter()
	lReceiver, err := createLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, mockLogsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	mockMetricsConsumer := exportertest.NewNopMetricsExporter()
	mReceiver, err := createMetricsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, mockMetricsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	mockTracesConsumer := exportertest.NewNopTraceExporter()
	tReceiver, err := createTraceReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, mockTracesConsumer)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, configmodels.Type("splunk_hec"), NewFactory().Type())
}

func TestCreateValidEndpoint(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:123")
	assert.NoError(t, err)
	assert.Equal(t, 123, endpoint)
}

func TestCreateInvalidEndpoint(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("")
	assert.EqualError(t, err, "endpoint is not formatted correctly: missing port in address")
	assert.Equal(t, 0, endpoint)
}

func TestCreateNoPort(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:")
	assert.EqualError(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"\": invalid syntax")
	assert.Equal(t, 0, endpoint)
}

func TestCreateLargePort(t *testing.T) {
	endpoint, err := extractPortFromEndpoint("localhost:65536")
	assert.EqualError(t, err, "port number must be between 1 and 65535")
	assert.Equal(t, 0, endpoint)
}

func TestValidate(t *testing.T) {
	err := createDefaultConfig().(*Config).validate()
	assert.NoError(t, err)
}

func TestValidateBadEndpoint(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.Endpoint = "localhost:abr"
	err := config.validate()
	assert.EqualError(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"abr\": invalid syntax")
}

func TestCreateNilNextConsumer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"

	mReceiver, err := createLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, nil)
	assert.EqualError(t, err, "nil metricsConsumer")
	assert.Nil(t, mReceiver, "receiver creation failed")
}

func TestCreateBadEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:abc"

	mockLogsConsumer := exportertest.NewNopLogsExporter()
	mReceiver, err := createLogsReceiver(context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()}, cfg, mockLogsConsumer)
	assert.EqualError(t, err, "endpoint port is not a number: strconv.ParseInt: parsing \"abc\": invalid syntax")
	assert.Nil(t, mReceiver, "receiver creation failed")
}
