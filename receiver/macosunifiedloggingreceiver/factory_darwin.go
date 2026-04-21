// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver/internal/metadata"
)

func newFactoryAdapter() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithLogs(createLogsReceiverDarwin, metadata.LogsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

// createLogsReceiver creates a logs receiver based on provided config
func createLogsReceiverDarwin(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	oCfg := cfg.(*Config)

	if err := oCfg.Validate(); err != nil {
		return nil, err
	}

	return newUnifiedLoggingReceiver(oCfg, set.Logger, consumer), nil
}
