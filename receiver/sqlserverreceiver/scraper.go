// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

const (
	computerNameKey = "computer_name"
	instanceNameKey = "sql_instance"
)

type sqlServerScraperHelper struct {
	id                 component.ID
	sqlQuery           string
	instanceName       string
	scrapeCfg          scraperhelper.ControllerConfig
	clientProviderFunc sqlquery.ClientProviderFunc
	dbProviderFunc     sqlquery.DbProviderFunc
	logger             *zap.Logger
	telemetry          sqlquery.TelemetryConfig
	client             sqlquery.DbClient
	db                 *sql.DB
	mb                 *metadata.MetricsBuilder
}

var _ scraper.Metrics = (*sqlServerScraperHelper)(nil)

func newSQLServerScraper(id component.ID,
	query string,
	instanceName string,
	scrapeCfg scraperhelper.ControllerConfig,
	logger *zap.Logger,
	telemetry sqlquery.TelemetryConfig,
	dbProviderFunc sqlquery.DbProviderFunc,
	clientProviderFunc sqlquery.ClientProviderFunc,
	mb *metadata.MetricsBuilder,
) *sqlServerScraperHelper {
	return &sqlServerScraperHelper{
		id:                 id,
		sqlQuery:           query,
		instanceName:       instanceName,
		scrapeCfg:          scrapeCfg,
		logger:             logger,
		telemetry:          telemetry,
		dbProviderFunc:     dbProviderFunc,
		clientProviderFunc: clientProviderFunc,
		mb:                 mb,
	}
}

func (s *sqlServerScraperHelper) ID() component.ID {
	return s.id
}

func (s *sqlServerScraperHelper) Start(context.Context, component.Host) error {
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open Db connection: %w", err)
	}
	s.client = s.clientProviderFunc(sqlquery.DbWrapper{Db: s.db}, s.sqlQuery, s.logger, s.telemetry)

	return nil
}

func (s *sqlServerScraperHelper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	var err error

	switch s.sqlQuery {
	case getSQLServerDatabaseIOQuery(s.instanceName):
		err = s.recordDatabaseIOMetrics(ctx)
	case getSQLServerPerformanceCounterQuery(s.instanceName):
		err = s.recordDatabasePerfCounterMetrics(ctx)
	case getSQLServerPropertiesQuery(s.instanceName):
		err = s.recordDatabaseStatusMetrics(ctx)
	default:
		return pmetric.Metrics{}, fmt.Errorf("Attempted to get metrics from unsupported query: %s", s.sqlQuery)
	}

	if err != nil {
		return pmetric.Metrics{}, err
	}

	return s.mb.Emit(), nil
}

func (s *sqlServerScraperHelper) Shutdown(_ context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *sqlServerScraperHelper) recordDatabaseIOMetrics(ctx context.Context) error {
	const databaseNameKey = "database_name"
	const physicalFilenameKey = "physical_filename"
	const logicalFilenameKey = "logical_filename"
	const fileTypeKey = "file_type"
	const readLatencyMsKey = "read_latency_ms"
	const writeLatencyMsKey = "write_latency_ms"
	const readCountKey = "reads"
	const writeCountKey = "writes"
	const readBytesKey = "read_bytes"
	const writeBytesKey = "write_bytes"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	var val float64
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverDatabaseName(row[databaseNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		val, err = strconv.ParseFloat(row[readLatencyMsKey], 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead)
		}

		val, err = strconv.ParseFloat(row[writeLatencyMsKey], 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite)
		}

		errs = append(errs, s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[readCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead))
		errs = append(errs, s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[writeCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite))
		errs = append(errs, s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[readBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead))
		errs = append(errs, s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[writeBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite))

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	if len(rows) == 0 {
		s.logger.Info("SQLServerScraperHelper: No rows found by query")
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabasePerfCounterMetrics(ctx context.Context) error {
	const counterKey = "counter"
	const valueKey = "value"
	// Constants are the columns for metrics from query
	const batchRequestRate = "Batch Requests/sec"
	const bufferCacheHitRatio = "Buffer cache hit ratio"
	const diskReadIOThrottled = "Disk Read IO Throttled/sec"
	const diskWriteIOThrottled = "Disk Write IO Throttled/sec"
	const lockWaits = "Lock Waits/sec"
	const processesBlocked = "Processes blocked"
	const sqlCompilationRate = "SQL Compilations/sec"
	const sqlReCompilationsRate = "SQL Re-Compilations/sec"
	const userConnCount = "User Connections"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		switch row[counterKey] {
		case batchRequestRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchRequestRateDataPoint(now, val)
			}
		case bufferCacheHitRatio:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(now, val)
			}
		case diskReadIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledReadRateDataPoint(now, row[valueKey]))
		case diskWriteIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledWriteRateDataPoint(now, row[valueKey]))
		case lockWaits:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitRateDataPoint(now, val)
			}
		case processesBlocked:
			errs = append(errs, s.mb.RecordSqlserverProcessesBlockedDataPoint(now, row[valueKey]))
		case sqlCompilationRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLCompilationRateDataPoint(now, val)
			}
		case sqlReCompilationsRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(now, val)
			}
		case userConnCount:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val)
			}
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabaseStatusMetrics(ctx context.Context) error {
	// Constants are the column names of the database status
	const dbOnline = "db_online"
	const dbRestoring = "db_restoring"
	const dbRecovering = "db_recovering"
	const dbPendingRecovery = "db_recoveryPending"
	const dbSuspect = "db_suspect"
	const dbOffline = "db_offline"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper failed getting metric rows: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOnline], metadata.AttributeDatabaseStatusOnline))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRestoring], metadata.AttributeDatabaseStatusRestoring))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRecovering], metadata.AttributeDatabaseStatusRecovering))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbPendingRecovery], metadata.AttributeDatabaseStatusPendingRecovery))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbSuspect], metadata.AttributeDatabaseStatusSuspect))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOffline], metadata.AttributeDatabaseStatusOffline))

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
}
