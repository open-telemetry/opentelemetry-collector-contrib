// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"
)

type logsJSONExporter struct {
	cfg       *Config
	logger    *zap.Logger
	db        driver.Conn
	insertSQL string
}

func newLogsJSONExporter(logger *zap.Logger, cfg *Config) *logsJSONExporter {
	return &logsJSONExporter{
		cfg:       cfg,
		logger:    logger,
		insertSQL: renderInsertLogsJSONSQL(cfg),
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
		if err := internal.CreateDatabase(ctx, e.db, e.cfg.database(), e.cfg.clusterString()); err != nil {
			return err
		}

		if err := createLogsJSONTable(ctx, e.cfg, e.db); err != nil {
			return err
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

		slLen := logs.ScopeLogs().Len()
		for j := range slLen {
			scopeLog := logs.ScopeLogs().At(j)
			scopeURL := scopeLog.SchemaUrl()
			scopeLogScope := scopeLog.Scope()
			scopeName := scopeLogScope.Name()
			scopeVersion := scopeLogScope.Version()
			scopeLogRecords := scopeLog.LogRecords()
			scopeAttrBytes, scopeAttrErr := json.Marshal(scopeLogScope.Attributes().AsRaw())
			if scopeAttrErr != nil {
				return fmt.Errorf("failed to marshal json log scope attributes: %w", scopeAttrErr)
			}

			slrLen := scopeLogRecords.Len()
			for k := range slrLen {
				r := scopeLogRecords.At(k)
				logAttrBytes, logAttrErr := json.Marshal(r.Attributes().AsRaw())
				if logAttrErr != nil {
					return fmt.Errorf("failed to marshal json log attributes: %w", logAttrErr)
				}

				timestamp := r.Timestamp()
				if timestamp == 0 {
					timestamp = r.ObservedTimestamp()
				}

				appendErr := batch.Append(
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

func renderInsertLogsJSONSQL(cfg *Config) string {
	return fmt.Sprintf(sqltemplates.LogsJSONInsert, cfg.database(), cfg.LogsTableName)
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
