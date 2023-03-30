// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func createReceiverFunc(sqlOpenerFunc sqlOpenerFunc, clientProviderFunc clientProviderFunc) receiver.CreateMetricsFunc {
	return func(
		ctx context.Context,
		settings receiver.CreateSettings,
		cfg component.Config,
		consumer consumer.Metrics,
	) (receiver.Metrics, error) {
		sqlCfg := cfg.(*Config)
		var opts []scraperhelper.ScraperControllerOption
		for i, query := range sqlCfg.Queries {
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
