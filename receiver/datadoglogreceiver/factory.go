// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadoglogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver/internal/metadata"
)

// NewFactory creates a factory for DataDog receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))

}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:8121",
		},
		ReadTimeout: 60 * time.Second,
	}
}

func createLogsReceiver(_ context.Context, params receiver.CreateSettings, cfg component.Config, consumer consumer.Logs) (r receiver.Logs, err error) {
	rcfg := cfg.(*Config)
	r = receivers.GetOrAdd(cfg, func() component.Component {
		dd, _ := newDataDogReceiver(rcfg, consumer, params)
		return dd
	})
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()
