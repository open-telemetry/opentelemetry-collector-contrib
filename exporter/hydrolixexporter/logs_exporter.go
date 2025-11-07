package hydrolixexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

type HydrolixLog struct {
	Timestamp             uint64                   `json:"timestamp"`
	ObservedTimestamp     uint64                   `json:"observed_timestamp,omitempty"`
	TraceID               string                   `json:"traceId,omitempty"`
	SpanID                string                   `json:"spanId,omitempty"`
	TraceFlags            uint32                   `json:"trace_flags,omitempty"`
	SeverityText          string                   `json:"severity_text,omitempty"`
	SeverityNumber        int32                    `json:"severity_number,omitempty"`
	Body                  string                   `json:"body,omitempty"`
	LogAttributes         []map[string]interface{} `json:"tags"`
	ResourceAttributes    []map[string]interface{} `json:"serviceTags"`
	ResourceSchemaUrl     string                   `json:"resource_schema_url,omitempty"`
	ScopeName             string                   `json:"scope_name,omitempty"`
	ScopeVersion          string                   `json:"scope_version,omitempty"`
	ScopeAttributes       []map[string]interface{} `json:"scope_attributes,omitempty"`
	ScopeDroppedAttrCount uint32                   `json:"scope_dropped_attr_count,omitempty"`
	ScopeSchemaUrl        string                   `json:"scope_schema_url,omitempty"`
	Flags                 uint32                   `json:"flags,omitempty"`
	ServiceName           string                   `json:"serviceName,omitempty"`
	HTTPStatusCode        string                   `json:"httpStatusCode,omitempty"`
	HTTPRoute             string                   `json:"httpRoute,omitempty"`
	HTTPMethod            string                   `json:"httpMethod,omitempty"`
}

func newLogsExporter(config *Config, set exporter.Settings) *logsExporter {
	return &logsExporter{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: set.Logger,
	}
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	logs := e.convertToHydrolixLogs(ld)

	jsonData, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hdx-table", e.config.HDXTable)
	req.Header.Set("x-hdx-transform", e.config.HDXTransform)
	req.SetBasicAuth(e.config.HDXUsername, e.config.HDXPassword)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Hydrolix: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("unexpected status code: %d (failed to read response body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	e.logger.Debug("successfully sent logs to Hydrolix",
		zap.Int("log_count", len(logs)),
		zap.String("table", e.config.HDXTable))

	return nil
}

func (e *logsExporter) convertToHydrolixLogs(ld plog.Logs) []HydrolixLog {
	var logs []HydrolixLog

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resource := rl.Resource()
		resourceAttrs := convertAttributes(resource.Attributes())
		resourceSchemaUrl := rl.SchemaUrl()

		serviceName := extractStringAttr(resource.Attributes(), "service.name")

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			scope := sl.Scope()
			scopeAttrs := convertAttributes(scope.Attributes())

			for k := 0; k < sl.LogRecords().Len(); k++ {
				logRecord := sl.LogRecords().At(k)

				hdxLog := HydrolixLog{
					Timestamp:             uint64(logRecord.Timestamp()),
					ObservedTimestamp:     uint64(logRecord.ObservedTimestamp()),
					TraceID:               logRecord.TraceID().String(),
					SpanID:                logRecord.SpanID().String(),
					TraceFlags:            uint32(logRecord.Flags()),
					SeverityText:          logRecord.SeverityText(),
					SeverityNumber:        int32(logRecord.SeverityNumber()),
					Body:                  logRecord.Body().AsString(),
					LogAttributes:         convertAttributes(logRecord.Attributes()),
					ResourceAttributes:    resourceAttrs,
					ResourceSchemaUrl:     resourceSchemaUrl,
					ScopeName:             scope.Name(),
					ScopeVersion:          scope.Version(),
					ScopeAttributes:       scopeAttrs,
					ScopeDroppedAttrCount: scope.DroppedAttributesCount(),
					ScopeSchemaUrl:        sl.SchemaUrl(),
					Flags:                 uint32(logRecord.Flags()),
					ServiceName:           serviceName,
					HTTPStatusCode:        extractStringAttr(logRecord.Attributes(), "http.response.status_code"),
					HTTPRoute:             extractStringAttr(logRecord.Attributes(), "http.route"),
					HTTPMethod:            extractStringAttr(logRecord.Attributes(), "http.request.method"),
				}

				logs = append(logs, hdxLog)
			}
		}
	}

	return logs
}
