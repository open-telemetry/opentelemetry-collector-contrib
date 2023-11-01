// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	stability = metadata.LogsStability
	typeStr   = metadata.Type
)

func createDefaultConfig() component.Config {
	return &Config{
		confignet.NetAddr{
			Endpoint: "0.0.0.0:9995",
		},
		map[string]string{},
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stability),
	)
}

func createLogsReceiver(ctx context.Context, params receiver.CreateSettings, basecfg component.Config, nextConsumer consumer.Logs) (receiver.Logs, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	receiverConfig := basecfg.(*Config)

	receiver := &sflowreceiverlogs{
		createSettings: params,
		config:         receiverConfig,
		nextConsumer:   nextConsumer,
	}

	return receiver, nil
}
