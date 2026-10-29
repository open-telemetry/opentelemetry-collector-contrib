// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	f := scraper.NewFactory(metadata.Type, nil,
		scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
			oCfg := cfg.(*Config)
			rcvr := newOsQueryReceiver(oCfg, set)
			return scraper.NewLogs(rcvr.collect)
		}, metadata.LogsStability))
	return scraperhelper.NewLogsController(
		&cfg.(*Config).ControllerConfig, set, consumer,
		scraperhelper.AddFactoryWithConfig(f, cfg),
	)
}
