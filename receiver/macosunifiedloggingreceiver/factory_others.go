// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"context"
	"errors"

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
		xreceiver.WithLogs(createLogsReceiverOthers, metadata.LogsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createLogsReceiverOthers(
	_ context.Context,
	_ receiver.Settings,
	_ component.Config,
	_ consumer.Logs,
) (receiver.Logs, error) {
	return nil, errors.New("macosunifiedloggingreceiver receiver is only supported on macOS")
}
