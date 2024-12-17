// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
)

type SQLOpenerFunc func(driverName, dataSourceName string) (*sql.DB, error)

type DbProviderFunc func() (*sql.DB, error)

type ClientProviderFunc func(Db, string, *zap.Logger, TelemetryConfig) DbClient

type Scraper struct {
	id                 component.ID
	Query              Query
	ScrapeCfg          scraperhelper.ControllerConfig
	StartTime          pcommon.Timestamp
	ClientProviderFunc ClientProviderFunc
	DbProviderFunc     DbProviderFunc
	Logger             *zap.Logger
	Telemetry          TelemetryConfig
	Client             DbClient
	Db                 *sql.DB
}

var _ scraper.Metrics = (*Scraper)(nil)

func NewScraper(id component.ID, query Query, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, telemetry TelemetryConfig, dbProviderFunc DbProviderFunc, clientProviderFunc ClientProviderFunc) *Scraper {
	return &Scraper{
		id:                 id,
		Query:              query,
		ScrapeCfg:          scrapeCfg,
		Logger:             logger,
		Telemetry:          telemetry,
		DbProviderFunc:     dbProviderFunc,
		ClientProviderFunc: clientProviderFunc,
	}
}

func (s *Scraper) ID() component.ID {
	return s.id
}

func (s *Scraper) Start(context.Context, component.Host) error {
	var err error
	s.Db, err = s.DbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open Db connection: %w", err)
	}
	s.Client = s.ClientProviderFunc(DbWrapper{s.Db}, s.Query.SQL, s.Logger, s.Telemetry)
	s.StartTime = pcommon.NewTimestampFromTime(time.Now())

	return nil
}

func (s *Scraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	out := pmetric.NewMetrics()
	rows, err := s.Client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, ErrNullValueWarning) {
			s.Logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return out, fmt.Errorf("Scraper: %w", err)
		}
	}
	ts := pcommon.NewTimestampFromTime(time.Now())
	rms := out.ResourceMetrics()
	rm := rms.AppendEmpty()
	sms := rm.ScopeMetrics()
	sm := sms.AppendEmpty()
	ms := sm.Metrics()
	var errs []error
	for _, metricCfg := range s.Query.Metrics {
		for i, row := range rows {
			if err = rowToMetric(row, metricCfg, ms.AppendEmpty(), s.StartTime, ts, s.ScrapeCfg); err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			}
		}
	}
	if errs != nil {
		return out, scrapererror.NewPartialScrapeError(errors.Join(errs...), len(errs))
	}
	return out, nil
}

func (s *Scraper) Shutdown(_ context.Context) error {
	if s.Db != nil {
		return s.Db.Close()
	}
	return nil
}
