// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !windows

// catchall for when the receiver is built for non-windows systems
package windowsservicereceiver

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func createMetricsReceiver(_ context.Context, _ receiver.Settings, _ component.Config, _ consumer.Metrics) (receiver.Metrics, error) {
	return nil, errors.New("WindowsServiceReceiver is only supported on Windows")
}
