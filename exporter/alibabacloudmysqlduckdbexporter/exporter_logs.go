// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const logsColumnCount = 15

type logsExporter struct {
	db               *sql.DB
	insertPrefix     string
	valuePlaceholder string

	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{
		logger:           logger,
		cfg:              cfg,
		insertPrefix:     internal.BuildInsertPrefix(sqltemplates.LogsInsert, cfg.Database, cfg.LogsTableName),
		valuePlaceholder: internal.BuildValuePlaceholder(logsColumnCount),
	}
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	dsn := e.cfg.buildDSN()

	if e.cfg.CreateSchema {
		initDB, err := internal.NewMySQLClientNoDB(dsn)
		if err != nil {
			return err
		}
		if err := internal.CreateDatabase(ctx, initDB, e.cfg.Database); err != nil {
			_ = initDB.Close()
			return err
		}
		_ = initDB.Close()
	}

	db, err := internal.NewMySQLClient(dsn)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.CreateSchema {
		ddl := renderCreateTableSQL(sqltemplates.LogsCreateTable, e.cfg.Database, e.cfg.LogsTableName)
		if err := internal.CreateTable(ctx, e.db, ddl); err != nil {
			return fmt.Errorf("create logs table: %w", err)
		}
	}

	if e.cfg.TTL > 0 {
		if err := internal.CreateTTLEvent(ctx, e.db, e.cfg.Database, e.cfg.LogsTableName, "timestamp", e.cfg.TTL); err != nil {
			return err
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
	start := time.Now()

	rows := make([][]any, 0, ld.LogRecordCount())

	rsLogs := ld.ResourceLogs()
	for i := 0; i < rsLogs.Len(); i++ {
		logs := rsLogs.At(i)
		res := logs.Resource()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resURL := logs.SchemaUrl()
		resAttrJSON := internal.AttributesToJSON(resAttr)

		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scope := scopeLog.Scope()
			scopeName := scope.Name()
			scopeVersion := scope.Version()
			scopeAttrJSON := internal.AttributesToJSON(scope.Attributes())

			for k := 0; k < scopeLog.LogRecords().Len(); k++ {
				r := scopeLog.LogRecords().At(k)

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				logAttrJSON := internal.AttributesToJSON(r.Attributes())

				rows = append(rows, []any{
					timestamp.AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint8(r.Flags()),
					r.SeverityText(),
					uint8(r.SeverityNumber()),
					serviceName,
					r.Body().AsString(),
					resURL,
					resAttrJSON,
					scopeURL,
					scopeName,
					scopeVersion,
					scopeAttrJSON,
					logAttrJSON,
				})
			}
		}
	}

	if err := internal.BatchInsert(ctx, e.db, e.insertPrefix, e.valuePlaceholder, rows, internal.DefaultBatchSize); err != nil {
		return fmt.Errorf("batch insert logs: %w", err)
	}

	duration := time.Since(start)
	e.logger.Debug("insert logs",
		zap.Int("records", len(rows)),
		zap.String("cost", duration.String()))

	return nil
}
