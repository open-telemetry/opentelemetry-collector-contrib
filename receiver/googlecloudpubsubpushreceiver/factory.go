// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		xreceiver.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextLogs consumer.Logs,
) (receiver.Logs, error) {
	return newPubSubPushReceiver(cfg.(*Config), set, nextLogs)
}
