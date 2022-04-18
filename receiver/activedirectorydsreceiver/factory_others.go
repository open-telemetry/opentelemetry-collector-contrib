//go:build !windows
// +build !windows

package activedirectorydsreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)


var errReceiverNotSupported = fmt.Errorf("The '%s' receiver is only supported on Windows", typeStr)

func createMetricsReceiver(
	_ context.Context,
	_ component.ReceiverCreateSettings,
	_ config.Receiver,
	_ consumer.Metrics,
) (component.MetricsReceiver, error) {
	return nil, errReceiverNotSupported
}
