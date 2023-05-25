// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type scraper struct {
	id                 component.ID
	query              Query
	scrapeCfg          scraperhelper.ScraperControllerSettings
	startTime          pcommon.Timestamp
	clientProviderFunc clientProviderFunc
	dbProviderFunc     dbProviderFunc
	logger             *zap.Logger
	client             dbClient
	db                 *sql.DB
}

var _ scraperhelper.Scraper = (*scraper)(nil)

func (s *scraper) ID() component.ID {
	return s.id
}

func (s *scraper) Start(context.Context, component.Host) error {
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open db connection: %w", err)
	}
	s.client = s.clientProviderFunc(dbWrapper{s.db}, s.query.SQL, s.logger)
	s.startTime = pcommon.NewTimestampFromTime(time.Now())

	return nil
}

func (s *scraper) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	out := pmetric.NewMetrics()
	rows, err := s.client.queryRows(ctx)
	if err != nil {
		if errors.Is(err, errNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return out, fmt.Errorf("scraper: %w", err)
		}
	}
	ts := pcommon.NewTimestampFromTime(time.Now())
	rms := out.ResourceMetrics()
	rm := rms.AppendEmpty()
	sms := rm.ScopeMetrics()
	sm := sms.AppendEmpty()
	ms := sm.Metrics()
	var errs error
	for _, metricCfg := range s.query.Metrics {
		for i, row := range rows {
			if err = rowToMetric(row, metricCfg, ms.AppendEmpty(), s.startTime, ts, s.scrapeCfg); err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = multierr.Append(errs, err)
			}
		}
	}
	if errs != nil {
		return out, scrapererror.NewPartialScrapeError(errs, len(multierr.Errors(errs)))
	}
	return out, nil
}

func (s *scraper) Shutdown(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
