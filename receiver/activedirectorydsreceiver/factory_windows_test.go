//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	t.Run("Nil config gives error", func(t *testing.T) {
		recv, err := createMetricsReceiver(
			context.Background(),
			componenttest.NewNopReceiverCreateSettings(),
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
			componenttest.NewNopReceiverCreateSettings(),
			createDefaultConfig(),
			&consumertest.MetricsSink{},
		)

		require.NoError(t, err)
		require.NotNil(t, recv)
	})
}
