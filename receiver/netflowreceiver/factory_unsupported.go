// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build solaris

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver/internal/metadata"
)

// NewFactory creates a factory for netflow receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		func() component.Config {
			return &Config{}
		},
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createLogsReceiver(context.Context, receiver.Settings, component.Config, consumer.Logs) (receiver.Logs, error) {
	return nil, errors.New("netflowreceiver is not supported on solaris")
}
