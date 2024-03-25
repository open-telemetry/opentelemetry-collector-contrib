// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func createLogsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	if err := component.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("cannot configure osquery logs receiver: %w", err)
	}
	receiver, err := newOsQueryReceiver(cfg.(*Config), consumer, set)
	if err != nil {
		return nil, fmt.Errorf("cannot create osquery logs receiver: %w", err)
	}
	return receiver, nil
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}
