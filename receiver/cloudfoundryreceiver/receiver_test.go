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

// Test to make sure a new receiver can be created properly, started and shutdown with the default config
func TestDefaultValidReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopCreateSettings()

	receiver, err := newCloudFoundryReceiver(
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

// Test to make sure start fails with invalid consumer
func TestInvalidConsumer(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopCreateSettings()

	receiver, err := newCloudFoundryReceiver(
		params,
		*cfg,
		nil,
	)

	require.EqualError(t, err, "nil next Consumer")
	require.Nil(t, receiver, "receiver creation failed")
}
