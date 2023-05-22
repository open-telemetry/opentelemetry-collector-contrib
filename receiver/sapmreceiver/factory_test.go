// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	params := receivertest.NewNopCreateSettings()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, nil)
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = ""
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with no endpoints must fail")
}

func TestCreateNoPort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = "localhost:"
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with no port number must fail")
}

func TestCreateLargePort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = "localhost:65536"
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with too large port number must fail")
}
