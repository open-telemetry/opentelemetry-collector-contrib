// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

// newFactoryAdapter creates a dummy factory for windowseventlog receiver
func newFactoryAdapter() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createLogsReceiver(
	_ context.Context,
	_ receiver.CreateSettings,
	_ component.Config,
	_ consumer.Logs,
) (receiver.Logs, error) {
	return nil, fmt.Errorf("windows eventlog receiver is only supported on Windows")
}
