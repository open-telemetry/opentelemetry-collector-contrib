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
	"go.uber.org/zap"
)

type sqlOpenerFunc func(driverName, dataSourceName string) (*sql.DB, error)

type dbProviderFunc func() (*sql.DB, error)

type clientProviderFunc func(db, string, *zap.Logger) dbClient

func createLogsReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateLogsFunc {
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

func createMetricsReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateMetricsFunc {
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
			mp := &scraper{
				id:        id,
				query:     query,
				scrapeCfg: sqlCfg.ScraperControllerSettings,
				logger:    settings.TelemetrySettings.Logger,
				dbProviderFunc: func() (*sql.DB, error) {
					return sqlOpenerFunc(sqlCfg.Driver, sqlCfg.DataSource)
				},
				clientProviderFunc: clientProviderFunc,
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
