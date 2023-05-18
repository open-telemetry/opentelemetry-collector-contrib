// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/metadata"
)

// NewFactory returns the component factory for the cloudflarereceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createLogsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	return newLogsReceiver(params, cfg, consumer)
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: LogsConfig{
			TimestampField: defaultTimestampField,
			TLS:            &configtls.TLSServerSetting{},
		},
	}
}
