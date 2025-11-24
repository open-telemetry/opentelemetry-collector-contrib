// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logsExporter struct {
	db             driver.Conn
	insertSQL      string
	schemaFeatures struct {
		EventName bool
	}

	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{
		logger: logger,
		cfg:    cfg,
	}
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	opt, err := e.cfg.buildClickHouseOptions()
	if err != nil {
		return err
	}

	e.db, err = internal.NewClickhouseClientFromOptions(opt)
	if err != nil {
		return err
	}

	if e.cfg.shouldCreateSchema() {
		if createDBErr := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); createDBErr != nil {
			return createDBErr
		}

		if createTableErr := createLogsTable(ctx, e.cfg, e.db); createTableErr != nil {
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		return fmt.Errorf("schema detection: %w", err)
	}

	e.renderInsertLogsSQL()

	return nil
}

const (
	logsColumnEventName = "EventName"
)

func (e *logsExporter) detectSchemaFeatures(ctx context.Context) error {
	columnNames, err := internal.GetTableColumns(ctx, e.db, e.cfg.database(), e.cfg.LogsTableName)
	if err != nil {
		return err
	}

	for _, name := range columnNames {
		if name == logsColumnEventName {
			e.schemaFeatures.EventName = true
		}
	}

	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		return e.db.Close()
	}

	return nil
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		if closeErr := batch.Close(); closeErr != nil {
			e.logger.Warn("failed to close logs batch", zap.Error(closeErr))
		}
	}(batch)

	processStart := time.Now()

	var logCount int
	rsLogs := ld.ResourceLogs()
	rsLen := rsLogs.Len()
	for i := range rsLen {
		logs := rsLogs.At(i)
		res := logs.Resource()
		resURL := logs.SchemaUrl()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrMap := internal.AttributesToMap(resAttr)

		slLen := logs.ScopeLogs().Len()
		for j := range slLen {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scopeLogScope := scopeLog.Scope()
			scopeName := scopeLogScope.Name()
			scopeVersion := scopeLogScope.Version()
			scopeLogRecords := scopeLog.LogRecords()
			scopeAttrMap := internal.AttributesToMap(scopeLogScope.Attributes())

			slrLen := scopeLogRecords.Len()
			for k := range slrLen {
				r := scopeLogRecords.At(k)
				logAttrMap := internal.AttributesToMap(r.Attributes())

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				columnValues := make([]any, 0, 16)
				columnValues = append(columnValues,
					timestamp.AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint8(r.Flags()),
					r.SeverityText(),
					uint8(r.SeverityNumber()),
					serviceName,
					r.Body().AsString(),
					resURL,
					resAttrMap,
					scopeURL,
					scopeName,
					scopeVersion,
					scopeAttrMap,
					logAttrMap,
				)

				if e.schemaFeatures.EventName {
					columnValues = append(columnValues, r.EventName())
				}

				appendErr := batch.Append(columnValues...)
				if appendErr != nil {
					return fmt.Errorf("failed to append log row: %w", appendErr)
				}

				logCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("logs insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert logs",
		zap.Int("records", logCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (e *logsExporter) renderInsertLogsSQL() {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.EventName {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsColumnEventName)

		featureColumnPositions.WriteString(", ?")
	}

	e.insertSQL = fmt.Sprintf(sqltemplates.LogsInsert, e.cfg.database(), e.cfg.LogsTableName, featureColumnNames.String(), featureColumnPositions.String())
}

func renderCreateLogsTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "TimestampTime")
	return fmt.Sprintf(sqltemplates.LogsCreateTable,
		cfg.database(), cfg.LogsTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func createLogsTable(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateLogsTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create logs table sql: %w", err)
	}

	return nil
}
