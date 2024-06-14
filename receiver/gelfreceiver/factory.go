// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gelfreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gelfreceiver/internal/metadata"
)

var (
	typeStr = component.MustNewType("gelfreceiver")
)

const (
	defaultListenAddress = "0.0.0.0:31250"
	defaultProtocol      = "udp"
)

// NewFactory returns the component factory for the cloudflarereceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevel(component.StabilityLevelBeta)),
	)
}

// func NewFactory() receiver.Factory {
// 	return nil
// }

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	// return nil, nil
	cfg := rConf.(*Config)
	rcvr, _ := newLogsReceiver(params, cfg, params.Logger, consumer)
	return rcvr, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		ListenAddress: string(defaultListenAddress),
		Protocol:      string(defaultProtocol),
	}
}
