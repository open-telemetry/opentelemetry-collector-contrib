// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
)

// anyLogsExporter is an interface that satisfies both the default map logsExporter and the logsJSONExporter
type anyLogsExporter interface {
	start(context.Context, component.Host) error
	shutdown(context.Context) error
	pushLogsData(context.Context, plog.Logs) error
}

type logsJSONExporter struct {
	cfg            *Config
	logger         *zap.Logger
	db             driver.Conn
	insertSQL      string
	schemaFeatures struct {
		AttributeKeys bool
		EventName     bool
	}
}

func newLogsJSONExporter(logger *zap.Logger, cfg *Config) *logsJSONExporter {
	return &logsJSONExporter{
		cfg:    cfg,
		logger: logger,
	}
}

func (e *logsJSONExporter) start(ctx context.Context, _ component.Host) error {
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

		if createTableErr := createLogsJSONTable(ctx, e.cfg, e.db); createTableErr != nil {
			return createTableErr
		}
	}

	err = e.detectSchemaFeatures(ctx)
	if err != nil {
		return fmt.Errorf("schema detection: %w", err)
	}

	e.renderInsertLogsJSONSQL()

	return nil
}

const (
	logsJSONColumnResourceAttributesKeys = "ResourceAttributesKeys"
	logsJSONColumnScopeAttributesKeys    = "ScopeAttributesKeys"
	logsJSONColumnLogAttributesKeys      = "LogAttributesKeys"
	logsJSONColumnEventName              = "EventName"
)

func (e *logsJSONExporter) detectSchemaFeatures(ctx context.Context) error {
	columnNames, err := internal.GetTableColumns(ctx, e.db, e.cfg.database(), e.cfg.LogsTableName)
	if err != nil {
		return err
	}

	for _, name := range columnNames {
		switch name {
		case logsJSONColumnResourceAttributesKeys:
			e.schemaFeatures.AttributeKeys = true
		case logsJSONColumnScopeAttributesKeys:
			e.schemaFeatures.AttributeKeys = true
		case logsJSONColumnLogAttributesKeys:
			e.schemaFeatures.AttributeKeys = true
		case logsJSONColumnEventName:
			e.schemaFeatures.EventName = true
		}
	}

	return nil
}

func (e *logsJSONExporter) shutdown(_ context.Context) error {
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			e.logger.Warn("failed to close json logs db connection", zap.Error(err))
			return err
		}
	}

	return nil
}

func (e *logsJSONExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	batch, err := e.db.PrepareBatch(ctx, e.insertSQL)
	if err != nil {
		return err
	}
	defer func(batch driver.Batch) {
		closeErr := batch.Close()
		if closeErr != nil {
			e.logger.Warn("failed to close json logs batch", zap.Error(closeErr))
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
		resAttrBytes, resAttrErr := json.Marshal(resAttr.AsRaw())
		if resAttrErr != nil {
			return fmt.Errorf("failed to marshal json log resource attributes: %w", resAttrErr)
		}

		var resAttrKeys []string
		if e.schemaFeatures.AttributeKeys {
			resAttrKeys = internal.UniqueFlattenedAttributes(resAttr)
		}

		slLen := logs.ScopeLogs().Len()
		for j := range slLen {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scopeLogScope := scopeLog.Scope()
			scopeName := scopeLogScope.Name()
			scopeVersion := scopeLogScope.Version()
			scopeLogRecords := scopeLog.LogRecords()
			scopeAttr := scopeLogScope.Attributes()
			scopeAttrBytes, scopeAttrErr := json.Marshal(scopeAttr.AsRaw())
			if scopeAttrErr != nil {
				return fmt.Errorf("failed to marshal json log scope attributes: %w", scopeAttrErr)
			}

			var scopeAttrKeys []string
			if e.schemaFeatures.AttributeKeys {
				scopeAttrKeys = internal.UniqueFlattenedAttributes(scopeAttr)
			}

			slrLen := scopeLogRecords.Len()
			for k := range slrLen {
				r := scopeLogRecords.At(k)
				logAttr := r.Attributes()
				logAttrBytes, logAttrErr := json.Marshal(logAttr.AsRaw())
				if logAttrErr != nil {
					return fmt.Errorf("failed to marshal json log attributes: %w", logAttrErr)
				}

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				columnValues := make([]any, 0, 19)
				columnValues = append(columnValues,
					timestamp.AsTime(),
					r.TraceID().String(),
					r.SpanID().String(),
					uint8(r.Flags()),
					r.SeverityText(),
					uint8(r.SeverityNumber()),
					serviceName,
					r.Body().AsString(),
					resURL,
					resAttrBytes,
					scopeURL,
					scopeName,
					scopeVersion,
					scopeAttrBytes,
					logAttrBytes,
				)

				if e.schemaFeatures.AttributeKeys {
					logAttrKeys := internal.UniqueFlattenedAttributes(logAttr)
					columnValues = append(columnValues, resAttrKeys, scopeAttrKeys, logAttrKeys)
				}

				if e.schemaFeatures.EventName {
					columnValues = append(columnValues, r.EventName())
				}

				appendErr := batch.Append(columnValues...)
				if appendErr != nil {
					return fmt.Errorf("failed to append json log row: %w", appendErr)
				}

				logCount++
			}
		}
	}

	processDuration := time.Since(processStart)
	networkStart := time.Now()
	if sendErr := batch.Send(); sendErr != nil {
		return fmt.Errorf("logs json insert failed: %w", sendErr)
	}

	networkDuration := time.Since(networkStart)
	totalDuration := time.Since(processStart)
	e.logger.Debug("insert json logs",
		zap.Int("records", logCount),
		zap.String("process_cost", processDuration.String()),
		zap.String("network_cost", networkDuration.String()),
		zap.String("total_cost", totalDuration.String()))

	return nil
}

func (e *logsJSONExporter) renderInsertLogsJSONSQL() {
	var featureColumnNames strings.Builder
	var featureColumnPositions strings.Builder

	if e.schemaFeatures.AttributeKeys {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsJSONColumnResourceAttributesKeys)
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsJSONColumnScopeAttributesKeys)
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsJSONColumnLogAttributesKeys)

		featureColumnPositions.WriteString(", ?, ?, ?")
	}

	if e.schemaFeatures.EventName {
		featureColumnNames.WriteString(", ")
		featureColumnNames.WriteString(logsJSONColumnEventName)

		featureColumnPositions.WriteString(", ?")
	}

	e.insertSQL = fmt.Sprintf(sqltemplates.LogsJSONInsert, e.cfg.database(), e.cfg.LogsTableName, featureColumnNames.String(), featureColumnPositions.String())
}

func renderCreateLogsJSONTableSQL(cfg *Config) string {
	ttlExpr := internal.GenerateTTLExpr(cfg.TTL, "Timestamp")
	return fmt.Sprintf(sqltemplates.LogsJSONCreateTable,
		cfg.database(), cfg.LogsTableName, cfg.clusterString(),
		cfg.tableEngineString(),
		ttlExpr,
	)
}

func createLogsJSONTable(ctx context.Context, cfg *Config, db driver.Conn) error {
	if err := db.Exec(ctx, renderCreateLogsJSONTableSQL(cfg)); err != nil {
		return fmt.Errorf("exec create logs json table sql: %w", err)
	}

	return nil
}
