// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	params := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "metrics receiver creation failed")
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	params := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateLogsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "logs receiver creation failed")
}
