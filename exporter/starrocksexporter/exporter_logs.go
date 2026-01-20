// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/sqltemplates"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logsExporter struct {
	db             *sql.DB
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
	db, err := GetDBClient(e.cfg)
	if err != nil {
		return err
	}
	e.db = db

	if e.cfg.shouldCreateSchema() {
		if createDBErr := internal.CreateDatabase(ctx, e.db, e.cfg.database()); createDBErr != nil {
			ReleaseDBClient(e.cfg)
			e.db = nil
			return createDBErr
		}

		if createTableErr := createLogsTable(ctx, e.cfg, e.db); createTableErr != nil {
			ReleaseDBClient(e.cfg)
			e.db = nil
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		ReleaseDBClient(e.cfg)
		e.db = nil
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
		err := ReleaseDBClient(e.cfg)
		e.db = nil
		return err
	}

	return nil
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	processStart := time.Now()

	var logCount int
	var values []string
	rsLogs := ld.ResourceLogs()
	rsLen := rsLogs.Len()

	for i := range rsLen {
		logs := rsLogs.At(i)
		res := logs.Resource()
		resURL := logs.SchemaUrl()
		resAttr := res.Attributes()
		serviceName := internal.GetServiceName(resAttr)
		resAttrJSON, err := internal.AttributesToJSON(resAttr)
		if err != nil {
			return fmt.Errorf("failed to convert resource attributes to JSON: %w", err)
		}

		slLen := logs.ScopeLogs().Len()
		for j := range slLen {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scopeLogScope := scopeLog.Scope()
			scopeName := scopeLogScope.Name()
			scopeVersion := scopeLogScope.Version()
			scopeLogRecords := scopeLog.LogRecords()
			scopeAttrJSON, err := internal.AttributesToJSON(scopeLogScope.Attributes())
			if err != nil {
				return fmt.Errorf("failed to convert scope attributes to JSON: %w", err)
			}

			slrLen := scopeLogRecords.Len()
			for k := range slrLen {
				r := scopeLogRecords.At(k)
				logAttrJSON, err := internal.AttributesToJSON(r.Attributes())
				if err != nil {
					return fmt.Errorf("failed to convert log attributes to JSON: %w", err)
				}

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				logTime := timestamp.AsTime()
				columnValues := make([]interface{}, 0, 16)
				columnValues = append(columnValues,
					serviceName,
					time.Time(logTime),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint8(r.Flags()),
					r.SeverityText(),
					uint8(r.SeverityNumber()),
					r.Body().AsString(),
					resURL,
					resAttrJSON,
					scopeURL,
					scopeName,
					scopeVersion,
					scopeAttrJSON,
					logAttrJSON,
				)

				if e.schemaFeatures.EventName {
					columnValues = append(columnValues, r.EventName())
				}

				values = append(values, internal.BuildValuesClause(columnValues))
				logCount++
			}
		}
	}

	if len(values) > 0 {
		// Build VALUES clause - replace the VALUES (?, ?, ...) part with actual values
		// Find the VALUES clause in the SQL
		valuesStart := strings.Index(e.insertSQL, "VALUES (")
		if valuesStart == -1 {
			return fmt.Errorf("failed to find VALUES clause in insert SQL")
		}
		// Replace from "VALUES (" onwards with the new VALUES clause
		insertSQL := e.insertSQL[:valuesStart] + "VALUES " + strings.Join(values, ",")
		_, execErr := e.db.ExecContext(ctx, insertSQL)
		if execErr != nil {
			return fmt.Errorf("failed to execute log insert: %w", execErr)
		}
	}

	processDuration := time.Since(processStart)
	e.logger.Debug("insert logs",
		zap.Int("records", logCount),
		zap.String("process_cost", processDuration.String()))

	return nil
}

func (e *logsExporter) renderInsertLogsSQL() {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.EventName {
		featureColumnNames.WriteString(", `")
		featureColumnNames.WriteString(logsColumnEventName)
		featureColumnNames.WriteString("`")

		featureColumnPositions.WriteString(", ?")
	}

	e.insertSQL = fmt.Sprintf(sqltemplates.LogsInsert, e.cfg.database(), e.cfg.LogsTableName, featureColumnNames.String(), featureColumnPositions.String())
}

func renderCreateLogsTableSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.LogsCreateTable,
		cfg.database(), cfg.LogsTableName,
	)
}

func createLogsTable(ctx context.Context, cfg *Config, db *sql.DB) error {
	sql := renderCreateLogsTableSQL(cfg)
	_, err := db.ExecContext(ctx, sql)
	if err != nil {
		return fmt.Errorf("exec create logs table sql: %w", err)
	}

	return nil
}
