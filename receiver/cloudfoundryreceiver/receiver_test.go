// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// Test to make sure a new metrics receiver can be created properly, started and shutdown with the default config
func TestDefaultValidMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopSettings()

	receiver, err := newCloudFoundryMetricsReceiver(
		params,
		*cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	// Test start
	ctx := context.Background()
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

// Test to make sure a new logs receiver can be created properly, started and shutdown with the default config
func TestDefaultValidLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopSettings()

	receiver, err := newCloudFoundryLogsReceiver(
		params,
		*cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	// Test start
	ctx := context.Background()
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}
