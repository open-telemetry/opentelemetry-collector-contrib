// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	t.Run("Nil config gives error", func(t *testing.T) {
		recv, err := createMetricsReceiver(
			context.Background(),
			receivertest.NewNopCreateSettings(),
			nil,
			&consumertest.MetricsSink{},
		)

		require.Nil(t, recv)
		require.Error(t, err)
		require.ErrorIs(t, err, errConfigNotActiveDirectory)
	})

	t.Run("Metrics receiver is created with default config", func(t *testing.T) {
		recv, err := createMetricsReceiver(
			context.Background(),
			receivertest.NewNopCreateSettings(),
			createDefaultConfig(),
			&consumertest.MetricsSink{},
		)

		require.NoError(t, err)
		require.NotNil(t, recv)

		// The receiver must be able to shutdown cleanly without a Start call.
		err = recv.Shutdown(context.Background())
		require.NoError(t, err)
	})
}
