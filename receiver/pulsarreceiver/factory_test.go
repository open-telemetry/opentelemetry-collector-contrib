// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		Topic:          "",
		ConsumerName:   defaultConsumerName,
		Subscription:   defaultSubscription,
		Endpoint:       defaultServiceURL,
		Authentication: Authentication{},
	}, cfg)
}

func Test_CreateTraceReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	recv, err := createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

func Test_CreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	recv, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

func Test_CreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	cfg.Endpoint = defaultServiceURL
	recv, err := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}
