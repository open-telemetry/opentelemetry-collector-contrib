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

//go:embed sql/logs_view.sql
var logsView string

// dLog Log to Doris
type dLog struct {
	ServiceName        string         `json:"service_name"`
	Timestamp          string         `json:"timestamp"`
	ServiceInstanceID  string         `json:"service_instance_id"`
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
		commonExporter: newExporter(logger, cfg, set, "LOG"),
	}
}

func (e *logsExporter) start(ctx context.Context, host component.Host) error {
	client, err := createDorisHTTPClient(ctx, e.cfg, host, e.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client

	if e.cfg.CreateSchema {
		conn, err := createDorisMySQLClient(e.cfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		err = createAndUseDatabase(ctx, conn, e.cfg)
		if err != nil {
			return err
		}

		ddl := fmt.Sprintf(logsDDL, e.cfg.Logs, e.cfg.propertiesStr())
		_, err = conn.ExecContext(ctx, ddl)
		if err != nil {
			return err
		}

		view := fmt.Sprintf(logsView, e.cfg.Logs, e.cfg.Logs)
		_, err = conn.ExecContext(ctx, view)
		if err != nil {
			e.logger.Warn("failed to create materialized view", zap.Error(err))
		}
	}

	go e.reporter.report()
	return nil
}

func (e *logsExporter) shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.CloseIdleConnections()
	}
	return nil
}

func (e *logsExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	label := generateLabel(e.cfg, e.cfg.Logs)
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
		serviceInstance := ""
		v, ok = resourceAttributes.Get(semconv.AttributeServiceInstanceID)
		if ok {
			serviceInstance = v.AsString()
		}

		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				log := &dLog{
					ServiceName:        serviceName,
					Timestamp:          e.formatTime(logRecord.Timestamp().AsTime()),
					ServiceInstanceID:  serviceInstance,
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

	return e.pushLogDataInternal(ctx, logs, label)
}

func (e *logsExporter) pushLogDataInternal(ctx context.Context, logs []*dLog, label string) error {
	marshal, err := toJSONLines(logs)
	if err != nil {
		return err
	}

	req, err := streamLoadRequest(ctx, e.cfg, e.cfg.Logs, marshal, label)
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

	if response.success() {
		e.reporter.incrTotalRows(int64(len(logs)))
		e.reporter.incrTotalBytes(int64(len(marshal)))

		if response.duplication() {
			e.logger.Warn("label already exists", zap.String("label", label), zap.Int("skipped", len(logs)))
		}

		if e.cfg.LogResponse {
			e.logger.Info("log response:\n" + string(body))
		} else {
			e.logger.Debug("log response:\n" + string(body))
		}
		return nil
	}

	return fmt.Errorf("failed to push log data, response:%s", string(body))
}
