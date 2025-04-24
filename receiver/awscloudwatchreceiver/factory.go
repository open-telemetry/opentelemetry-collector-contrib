// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

// NewFactory returns the component factory for the awscloudwatchreceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	rcvr := newLogsReceiver(cfg, settings, consumer)
	return rcvr, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: &LogsConfig{
			PollInterval:        defaultPollInterval,
			MaxEventsPerRequest: defaultEventLimit,
			Groups: GroupConfig{
				AutodiscoverConfig: &AutodiscoverConfig{
					Limit: defaultLogGroupLimit,
				},
			},
		},
	}
}
