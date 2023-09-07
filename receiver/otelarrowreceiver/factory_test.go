// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/otel-arrow/collector/testutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)
	cfg.HTTP.Endpoint = testutil.GetAvailableLocalAddress(t)

	creationSet := receivertest.NewNopCreateSettings()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.Nil(t, tReceiver)
	assert.Error(t, err)
	assert.Contains(t, "unimplemented", err.Error())
}
