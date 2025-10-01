// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"
)

type SQLOpenerFunc func(driverName, dataSourceName string) (*sql.DB, error)

type DbProviderFunc func() (*sql.DB, error)

type ClientProviderFunc func(Db, string, *zap.Logger, TelemetryConfig) DbClient

type Scraper struct {
	id                   component.ID
	Query                Query
	ScrapeCfg            scraperhelper.ControllerConfig
	StartTime            pcommon.Timestamp
	ClientProviderFunc   ClientProviderFunc
	DbProviderFunc       DbProviderFunc
	Logger               *zap.Logger
	Telemetry            TelemetryConfig
	Client               DbClient
	Db                   *sql.DB
	InstrumentationScope pcommon.InstrumentationScope
}

var _ scraper.Metrics = (*Scraper)(nil)

func NewScraper(id component.ID, query Query, scrapeCfg scraperhelper.ControllerConfig, logger *zap.Logger, telemetry TelemetryConfig, dbProviderFunc DbProviderFunc, clientProviderFunc ClientProviderFunc, instrumentationScope pcommon.InstrumentationScope) *Scraper {
	return &Scraper{
		id:                   id,
		Query:                query,
		ScrapeCfg:            scrapeCfg,
		Logger:               logger,
		Telemetry:            telemetry,
		DbProviderFunc:       dbProviderFunc,
		ClientProviderFunc:   clientProviderFunc,
		InstrumentationScope: instrumentationScope,
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
		if !errors.Is(err, ErrNullValueWarning) {
			return out, fmt.Errorf("Scraper: %w", err)
		}
		s.Logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}
	ts := pcommon.NewTimestampFromTime(time.Now())
	rms := out.ResourceMetrics()
	rm := rms.AppendEmpty()
	sms := rm.ScopeMetrics()
	sm := sms.AppendEmpty()
	s.InstrumentationScope.CopyTo(sm.Scope())
	ms := sm.Metrics()
	var errs []error
	for i := range s.Query.Metrics {
		metricCfg := &s.Query.Metrics[i]
		for j, row := range rows {
			if err = rowToMetric(row, metricCfg, ms.AppendEmpty(), s.StartTime, ts, s.ScrapeCfg); err != nil {
				err = fmt.Errorf("row %d: %w", j, err)
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

func BuildDataSourceString(config Config) (string, error) {
	var auth string
	if config.Username != "" {
		// MySQL doesn't need URL escaping
		if config.Driver == DriverMySQL {
			auth = fmt.Sprintf("%s:%s@", config.Username, string(config.Password))
		} else {
			auth = fmt.Sprintf("%s:%s@", url.QueryEscape(config.Username), url.QueryEscape(string(config.Password)))
		}
	}

	query := url.Values{}
	for k, v := range config.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	var connStr string
	switch config.Driver {
	case DriverHDB:
		// HDB connection string format: hdb://user:pass@host:port?param1=value1
		connStr = fmt.Sprintf("hdb://%s%s:%d", auth, config.Host, config.Port)
	case DriverMySQL:
		// MySQL connection string format: user:pass@tcp(host:port)/db?param1=value1&param2=value2
		connStr = fmt.Sprintf("%stcp(%s:%d)/%s", auth, config.Host, config.Port, config.Database)
	case DriverOracle:
		// Oracle connection string format: oracle://user:pass@host:port/service_name?param1=value1&param2=value2
		connStr = fmt.Sprintf("oracle://%s%s:%d/%s", auth, config.Host, config.Port, config.Database)
	case DriverPostgres:
		// PostgreSQL connection string format: postgresql://user:pass@host:port/db?param1=value1&param2=value2
		connStr = fmt.Sprintf("postgresql://%s%s:%d/%s", auth, config.Host, config.Port, config.Database)
	case DriverSnowflake:
		// Snowflake connection string format: user:pass@host:port/database?param1=value1&param2=value2
		connStr = fmt.Sprintf("%s%s:%d/%s", auth, config.Host, config.Port, config.Database)
	case DriverSQLServer:
		// SQL Server connection string format: sqlserver://username:password@host:port/instance

		// replace all backslashes with forward slashes
		host := strings.ReplaceAll(config.Host, "\\", "/")
		// if host contains a "/", split it into hostname and instance
		parts := strings.SplitN(host, "/", 2)
		hostname := parts[0]
		query.Set("database", config.Database)
		if len(parts) > 1 {
			connStr = fmt.Sprintf("sqlserver://%s%s:%d/%s", auth, hostname, config.Port, parts[1])
		} else {
			connStr = fmt.Sprintf("sqlserver://%s%s:%d", auth, hostname, config.Port)
		}
	case DriverTDS:
		// TDS connection string format: tds://user:pass@host:port/database
		connStr = fmt.Sprintf("tds://%s%s:%d/%s", auth, config.Host, config.Port, config.Database)
	default:
		return "", fmt.Errorf("unsupported driver: %s", config.Driver)
	}

	// Append query parameters if any exist
	if len(query) > 0 {
		connStr = fmt.Sprintf("%s?%s", connStr, query.Encode())
	}

	return connStr, nil
}
