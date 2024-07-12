// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"
)

const (
	logsDDL = `
CREATE TABLE IF NOT EXISTS %s
(
    service_name          VARCHAR(200),
    timestamp             DATETIME(6),
    trace_id              VARCHAR(200),
    span_id               STRING,
    severity_number       INT,
    severity_text         STRING,
    body                  STRING,
    resource_attributes   VARIANT,
    log_attributes        VARIANT,
    scope_name            STRING,
    scope_version         STRING,

    INDEX idx_service_name(service_name) USING INVERTED,
    INDEX idx_timestamp(timestamp) USING INVERTED,
    INDEX idx_trace_id(trace_id) USING INVERTED,
    INDEX idx_span_id(span_id) USING INVERTED,
    INDEX idx_severity_number(severity_number) USING INVERTED,
    INDEX idx_body(body) USING INVERTED PROPERTIES("parser"="unicode", "support_phrase"="true"),
    INDEX idx_severity_text(severity_text) USING INVERTED,
    INDEX idx_resource_attributes(resource_attributes) USING INVERTED,
    INDEX idx_log_attributes(log_attributes) USING INVERTED,
    INDEX idx_scope_name(scope_name) USING INVERTED,
    INDEX idx_scope_version(scope_version) USING INVERTED
)
ENGINE = OLAP
DUPLICATE KEY(service_name, timestamp)
PARTITION BY RANGE(timestamp) ()
DISTRIBUTED BY HASH(trace_id) BUCKETS AUTO
PROPERTIES (
"replication_num" = "1",
"compaction_policy" = "time_series",
"enable_single_replica_compaction" = "true",
"dynamic_partition.enable" = "true",
"dynamic_partition.create_history_partition" = "true",
"dynamic_partition.time_unit" = "DAY",
"dynamic_partition.start" = "%d",
"dynamic_partition.history_partition_num" = "%d",
"dynamic_partition.end" = "1",
"dynamic_partition.prefix" = "p"
);
`
)

// dLog Log to Doris
type dLog struct {
	ServiceName        string         `json:"service_name"`
	Timestamp          string         `json:"timestamp"`
	TraceID            string         `json:"trace_id"`
	SpanID             string         `json:"span_id"`
	SeverityNumber     int32          `json:"severity_number"`
	SeverityText       string         `json:"severity_text"`
	Body               string         `json:"body"`
	ResourceAttributes map[string]any `json:"resource_attributes"`
	LogAttributes      map[string]any `json:"log_attributes"`
	ScopeName          string         `json:"scope_name"`
	ScopeVersion       string         `json:"scope_version"`
}

type logsExporter struct {
	*commonExporter
}

func newLogsExporter(logger *zap.Logger, cfg *Config) (*logsExporter, error) {
	commonExporter, err := newExporter(logger, cfg)
	if err != nil {
		return nil, err
	}
	return &logsExporter{
		commonExporter: commonExporter,
	}, nil
}

func (e *logsExporter) start(ctx context.Context, _ component.Host) error {
	if !e.cfg.CreateSchema {
		return nil
	}

	conn, err := createMySQLClient(e.cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = createAndUseDatabase(ctx, conn, e.cfg)
	if err != nil {
		return err
	}

	ddl := fmt.Sprintf(logsDDL, e.cfg.Table.Logs, e.cfg.start(), e.cfg.CreateHistoryDays)
	_, err = conn.ExecContext(ctx, ddl)
	return err
}

func (e *logsExporter) shutdown(ctx context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}

func (e *logsExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	logs := make([]*dLog, 0, ld.LogRecordCount())

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLogs := ld.ResourceLogs().At(i)
		resource := resourceLogs.Resource()
		resourceAttributes := resource.Attributes()
		serviceName := ""
		v, ok := resourceAttributes.Get(semconv.AttributeServiceName)
		if ok {
			serviceName = v.AsString()
		}

		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				log := &dLog{
					ServiceName:        serviceName,
					Timestamp:          e.formatTime(logRecord.Timestamp().AsTime()),
					TraceID:            traceutil.TraceIDToHexOrEmptyString(logRecord.TraceID()),
					SpanID:             traceutil.SpanIDToHexOrEmptyString(logRecord.SpanID()),
					SeverityNumber:     int32(logRecord.SeverityNumber()),
					SeverityText:       logRecord.SeverityText(),
					Body:               logRecord.Body().AsString(),
					ResourceAttributes: resourceAttributes.AsRaw(),
					LogAttributes:      logRecord.Attributes().AsRaw(),
					ScopeName:          scopeLogs.Scope().Name(),
					ScopeVersion:       scopeLogs.Scope().Version(),
				}

				logs = append(logs, log)
			}

		}

	}

	return e.pushLogDataInternal(ctx, logs)
}

func (e *logsExporter) pushLogDataInternal(ctx context.Context, logs []*dLog) error {
	marshal, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Table.Logs, marshal)
	if err != nil {
		return err
	}

	res, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	response := streamLoadResponse{}
	json.Unmarshal(body, &response)

	if !response.success() {
		return fmt.Errorf("failed to push log data: %s", response.Message)
	}

	return nil
}
