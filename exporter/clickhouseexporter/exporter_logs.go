// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ClickHouse/clickhouse-go/v2/lib/proto"
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

		if createTableErr := createLogsTable(ctx, e.cfg, e.db, e.logger); createTableErr != nil {
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		e.logger.Error("schema detection failed", zap.Error(err))
	}

	if err := e.renderInsertLogsSQL(); err != nil {
		return fmt.Errorf("render logs insert sql: %w", err)
	}

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
			// 16 matches the max number of columns in the insert statement.
			// If you add or remove columns, update this value.
			columnValues := make([]any, 0, 16)
			for k := range slrLen {
				r := scopeLogRecords.At(k)
				logAttrMap := internal.AttributesToMap(r.Attributes())

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				columnValues = columnValues[:0]
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

func (e *logsExporter) renderInsertLogsSQL() error {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.EventName {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsColumnEventName)

		featureColumnPositions.WriteString(", ?")
	}

	data := sqltemplates.InsertData{
		Database:               e.cfg.database(),
		TableName:              e.cfg.LogsTableName,
		FeatureColumnNames:     featureColumnNames.String(),
		FeatureColumnPositions: featureColumnPositions.String(),
	}

	var buf bytes.Buffer
	if err := sqltemplates.LogsInsertTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute logs insert template: %w", err)
	}

	e.insertSQL = buf.String()
	return nil
}

// versionFullTextSearch is the minimum ClickHouse version that supports TYPE text() indexes.
var versionFullTextSearch = proto.Version{Major: 26, Minor: 2}

func renderCreateLogsTableSQL(cfg *Config, hasFullTextSearch bool) (string, error) {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "toDateTime(Timestamp)")
	data := sqltemplates.CreateTableData{
		Database:          cfg.database(),
		TableName:         cfg.LogsTableName,
		ClusterString:     cfg.clusterString(),
		Engine:            cfg.tableEngineString(),
		TTL:               ttlExpr,
		HasFullTextSearch: hasFullTextSearch,
	}

	var buf bytes.Buffer
	if err := sqltemplates.LogsCreateTableTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute logs create table template: %w", err)
	}

	return buf.String(), nil
}

func createLogsTable(ctx context.Context, cfg *Config, db driver.Conn, logger *zap.Logger) error {
	hasFullTextSearch := false
	sv, err := db.ServerVersion()
	if err != nil {
		logger.Warn("failed to get ClickHouse server version, falling back to bloom filter indexes", zap.Error(err))
	} else {
		hasFullTextSearch = proto.CheckMinVersion(versionFullTextSearch, sv.Version)
	}

	sql, err := renderCreateLogsTableSQL(cfg, hasFullTextSearch)
	if err != nil {
		return fmt.Errorf("render create logs table sql: %w", err)
	}

	if err := db.Exec(ctx, sql); err != nil {
		return fmt.Errorf("exec create logs table sql: %w", err)
	}

	return nil
}
