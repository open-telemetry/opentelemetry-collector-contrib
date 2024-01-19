// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sqlquery"
)

func createLogsReceiverFunc(sqlOpenerFunc sqlquery.SqlOpenerFunc, clientProviderFunc sqlquery.ClientProviderFunc) receiver.CreateLogsFunc {
	return func(
		ctx context.Context,
		settings receiver.CreateSettings,
		config component.Config,
		consumer consumer.Logs,
	) (receiver.Logs, error) {
		sqlQueryConfig := config.(*Config)
		return newLogsReceiver(sqlQueryConfig, settings, sqlOpenerFunc, clientProviderFunc, consumer)
	}
}

func createMetricsReceiverFunc(sqlOpenerFunc sqlquery.SqlOpenerFunc, clientProviderFunc sqlquery.ClientProviderFunc) receiver.CreateMetricsFunc {
	return func(
		ctx context.Context,
		settings receiver.CreateSettings,
		cfg component.Config,
		consumer consumer.Metrics,
	) (receiver.Metrics, error) {
		sqlCfg := cfg.(*Config)
		var opts []scraperhelper.ScraperControllerOption
		for i, query := range sqlCfg.Queries {
			if len(query.Metrics) == 0 {
				continue
			}
			id := component.NewIDWithName("sqlqueryreceiver", fmt.Sprintf("query-%d: %s", i, query.SQL))
			mp := &sqlquery.Scraper{
				Id:        id,
				Query:     query,
				ScrapeCfg: sqlCfg.ScraperControllerSettings,
				Logger:    settings.TelemetrySettings.Logger,
				Telemetry: sqlCfg.Config.Telemetry,
				DbProviderFunc: func() (*sql.DB, error) {
					return sqlOpenerFunc(sqlCfg.Driver, sqlCfg.DataSource)
				},
				ClientProviderFunc: clientProviderFunc,
			}
			opt := scraperhelper.AddScraper(mp)
			opts = append(opts, opt)
		}
		return scraperhelper.NewScraperControllerReceiver(
			&sqlCfg.ScraperControllerSettings,
			settings,
			consumer,
			opts...,
		)
	}
}
