//go:build !windows
// +build !windows

package activedirectorydsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	t.Parallel()
	
	recv, err := createMetricsReceiver(context.Background(), component.ReceiverCreateSettings{}, &Config{}, &consumertest.MetricsSink{})
	require.Nil(t, recv)
	require.ErrorIs(t, err, errReceiverNotSupported)
}
