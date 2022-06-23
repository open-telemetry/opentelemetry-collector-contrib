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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type scraper struct {
	id                 config.ComponentID
	query              Query
	clientProviderFunc clientProviderFunc
	dbProviderFunc     dbProviderFunc
	logger             *zap.Logger
	client             dbClient
	db                 *sql.DB
}

var _ scraperhelper.Scraper = (*scraper)(nil)

func (s scraper) ID() config.ComponentID {
	return s.id
}

func (s *scraper) Start(context.Context, component.Host) error {
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	s.client = s.clientProviderFunc(s.db, s.query.SQL, s.logger)
	return nil
}

func (s scraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	out := pmetric.NewMetrics()
	rows, err := s.client.metricRows(ctx)
	if err != nil {
		return out, fmt.Errorf("scraper: %w", err)
	}
	rms := out.ResourceMetrics()
	rm := rms.AppendEmpty()
	sms := rm.ScopeMetrics()
	sm := sms.AppendEmpty()
	ms := sm.Metrics()
	var errs error
	for _, metricCfg := range s.query.Metrics {
		for i, row := range rows {
			if err = rowToMetric(row, metricCfg, ms.AppendEmpty()); err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = multierr.Append(errs, err)
			}
		}
	}
	if errs != nil {
		errs = fmt.Errorf("scraper.Scrape row conversion errors: %w", errs)
	}
	return out, errs
}

func (s scraper) Shutdown(ctx context.Context) error {
	return s.db.Close()
}
