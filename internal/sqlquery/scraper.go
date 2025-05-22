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

func BuildDataSourceString(driver string, dataSource DataSourceConfig) (string, error) {
	switch driver {
	case "postgres":
		return buildPostgreSQLString(dataSource)
	case "mysql":
		return buildMySQLString(dataSource)
	case "snowflake":
		return buildSnowflakeString(dataSource)
	case "sqlserver":
		return buildSQLServerString(dataSource)
	case "oracle":
		return buildOracleString(dataSource)
	default:
		return "", fmt.Errorf("unsupported driver: %s", driver)
	}
}

func buildPostgreSQLString(conn DataSourceConfig) (string, error) {
	// PostgreSQL connection string format: postgresql://user:pass@host:port/db?param1=value1&param2=value2
	var auth string
	if conn.Username != "" {
		auth = fmt.Sprintf("%s:%s@", url.QueryEscape(conn.Username), url.QueryEscape(string(conn.Password)))
	}

	query := url.Values{}
	for k, v := range conn.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	connStr := fmt.Sprintf("postgresql://%s%s:%d/%s", auth, conn.Host, conn.Port, conn.Database)
	if len(query) > 0 {
		connStr += "?" + query.Encode()
	}

	return connStr, nil
}

func buildMySQLString(conn DataSourceConfig) (string, error) {
	// MySQL connection string format: user:pass@tcp(host:port)/db?param1=value1&param2=value2
	var auth string

	// MySQL requires no escaping of username and password
	if conn.Username != "" {
		username := conn.Username
		password := string(conn.Password)
		auth = fmt.Sprintf("%s:%s@", username, password)
	}

	query := url.Values{}
	for k, v := range conn.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	connStr := fmt.Sprintf("%stcp(%s:%d)/%s", auth, conn.Host, conn.Port, conn.Database)
	if len(query) > 0 {
		connStr += "?" + query.Encode()
	}

	return connStr, nil
}

func buildSnowflakeString(conn DataSourceConfig) (string, error) {
	// Snowflake connection string format: user:pass@host:port/database?param1=value1&param2=value2
	var auth string
	if conn.Username != "" {
		auth = fmt.Sprintf("%s:%s@", url.QueryEscape(conn.Username), url.QueryEscape(string(conn.Password)))
	}

	query := url.Values{}
	for k, v := range conn.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	connStr := fmt.Sprintf("%s%s:%d/%s", auth, conn.Host, conn.Port, conn.Database)
	if len(query) > 0 {
		connStr += "?" + query.Encode()
	}

	return connStr, nil
}

func buildSQLServerString(conn DataSourceConfig) (string, error) {
	// SQL Server connection string format: sqlserver://user:pass@host/instance?param1=value1&param2=value2
	var auth string
	if conn.Username != "" {
		auth = fmt.Sprintf("%s:%s@", url.QueryEscape(conn.Username), url.QueryEscape(string(conn.Password)))
	}

	query := url.Values{}
	query.Set("database", conn.Database)
	for k, v := range conn.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	// Replace backslashes with forward slashes in hostname
	host := strings.ReplaceAll(conn.Host, "\\", "/")
	connStr := fmt.Sprintf("sqlserver://%s%s:%d", auth, host, conn.Port)
	if len(query) > 0 {
		connStr += "?" + query.Encode()
	}

	return connStr, nil
}

func buildOracleString(conn DataSourceConfig) (string, error) {
	// Oracle connection string format: oracle://user:pass@host:port/service_name?param1=value1&param2=value2
	var auth string
	if conn.Username != "" {
		auth = fmt.Sprintf("%s:%s@", url.QueryEscape(conn.Username), url.QueryEscape(string(conn.Password)))
	}

	query := url.Values{}
	for k, v := range conn.AdditionalParams {
		query.Set(k, fmt.Sprintf("%v", v))
	}

	connStr := fmt.Sprintf("oracle://%s%s:%d/%s", auth, conn.Host, conn.Port, conn.Database)
	if len(query) > 0 {
		connStr += "?" + query.Encode()
	}

	return connStr, nil
}
