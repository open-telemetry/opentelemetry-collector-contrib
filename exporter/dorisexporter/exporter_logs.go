// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	_ "embed" // for SQL file embedding
	"encoding/json"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

//go:embed sql/logs_ddl.sql
var logsDDL string

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

func newLogsExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings) *logsExporter {
	return &logsExporter{
		commonExporter: newExporter(logger, cfg, set),
	}
}

func (e *logsExporter) start(ctx context.Context, host component.Host) error {
	client, err := createDorisHTTPClient(ctx, e.cfg, host, e.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client

	if !e.cfg.CreateSchema {
		return nil
	}

	conn, err := createDorisMySQLClient(e.cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	err = createAndUseDatabase(ctx, conn, e.cfg)
	if err != nil {
		return err
	}

	ddl := fmt.Sprintf(logsDDL, e.cfg.Table.Logs, e.cfg.propertiesStr())
	_, err = conn.ExecContext(ctx, ddl)
	return err
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
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
	marshal, err := toJsonLines(logs)
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
	err = json.Unmarshal(body, &response)
	if err != nil {
		return err
	}

	if !response.success() {
		return fmt.Errorf("failed to push log data: %s", response.Message)
	}

	return nil
}
