// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fiddlerreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFactoryCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default configuration")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	fiddlerCfg, ok := cfg.(*Config)
	assert.True(t, ok, "configuration is not of type Config")
	assert.Equal(t, defaultInterval, fiddlerCfg.Interval)
}

func TestCreateReceiver_Factory(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "https://api.fiddler.ai"
	cfg.Token = "test-token"

	consumer := consumertest.NewNop()
	receiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(factory.Type()),
		cfg,
		consumer,
	)

	assert.NoError(t, err)
	assert.NotNil(t, receiver)
}
