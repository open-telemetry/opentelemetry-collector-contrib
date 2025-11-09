package hydrolixexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

type HydrolixSpan struct {
	TraceID            string                   `json:"traceId"`
	SpanID             string                   `json:"spanId"`
	Name               string                   `json:"name"`
	ParentSpanID       string                   `json:"parentSpanId,omitempty"`
	ServiceName        string                   `json:"serviceName,omitempty"`
	Duration           uint64                   `json:"duration"`
	SpanAttributes     []map[string]interface{} `json:"tags"`
	ResourceAttributes []map[string]interface{} `json:"serviceTags"`
	HTTPStatusCode     string                   `json:"httpStatusCode,omitempty"`
	SpanKind           string                   `json:"spanKind"`
	SpanKindString     string                   `json:"spanKindString"`
	Timestamp          uint64                   `json:"timestamp"`
	StartTime          uint64                   `json:"startTime"`
	EndTime            uint64                   `json:"endTime"`
	HTTPRoute          string                   `json:"httpRoute,omitempty"`
	Logs               []TraceLog               `json:"logs,omitempty"`
	StatusCodeString   string                   `json:"statusCodeString"`
	StatusCode         string                   `json:"statusCode"`
	StatusMessage      string                   `json:"statusMessage,omitempty"`
	HTTPMethod         string                   `json:"httpMethod,omitempty"`
}

type TraceLog struct {
	Name      string                   `json:"name,omitempty"`
	Timestamp uint64                   `json:"timestamp"`
	Field     []map[string]interface{} `json:"field"`
}

func newTracesExporter(config *Config, set exporter.Settings) *tracesExporter {
	return &tracesExporter{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: set.Logger,
	}
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	spans := e.convertToHydrolixSpans(td)

	jsonData, err := json.Marshal(spans)
	if err != nil {
		return fmt.Errorf("failed to marshal spans: %w", err)
	}

	return e.sendToHydrolix(ctx, jsonData, e.config.HDXTable, e.config.HDXTransform)
}

func (e *tracesExporter) convertToHydrolixSpans(td ptrace.Traces) []HydrolixSpan {
	var spans []HydrolixSpan

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resource := rs.Resource()
		resourceAttrs := convertAttributes(resource.Attributes())

		serviceName := ""
		if sn, ok := resource.Attributes().Get("service.name"); ok {
			serviceName = sn.Str()
		}

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				startTime := uint64(span.StartTimestamp())
				endTime := uint64(span.EndTimestamp())
				duration := uint64(0)
				if endTime > startTime {
					duration = endTime - startTime
				}

				spanAttrs := convertAttributes(span.Attributes())

				httpStatus := extractStringAttr(span.Attributes(), "http.response.status_code")
				httpRoute := extractStringAttr(span.Attributes(), "http.route")
				httpMethod := extractStringAttr(span.Attributes(), "http.request.method")

				logs := convertEvents(span.Events())

				hydrolixSpan := HydrolixSpan{
					TraceID:            span.TraceID().String(),
					SpanID:             span.SpanID().String(),
					Name:               span.Name(),
					ParentSpanID:       span.ParentSpanID().String(),
					ServiceName:        serviceName,
					Duration:           duration,
					SpanAttributes:     spanAttrs,
					ResourceAttributes: resourceAttrs,
					HTTPStatusCode:     httpStatus,
					SpanKind:           fmt.Sprintf("%d", span.Kind()),
					SpanKindString:     getSpanKindString(span.Kind()),
					Timestamp:          startTime,
					StartTime:          startTime,
					EndTime:            endTime,
					HTTPRoute:          httpRoute,
					Logs:               logs,
					StatusCodeString:   getStatusCodeString(span.Status().Code()),
					StatusCode:         fmt.Sprintf("%d", span.Status().Code()),
					StatusMessage:      span.Status().Message(),
					HTTPMethod:         httpMethod,
				}

				spans = append(spans, hydrolixSpan)
			}
		}
	}

	return spans
}

func convertEvents(events ptrace.SpanEventSlice) []TraceLog {
	if events.Len() == 0 {
		return nil
	}

	logs := make([]TraceLog, 0, events.Len())

	for i := 0; i < events.Len(); i++ {
		event := events.At(i)

		log := TraceLog{
			Name:      event.Name(),
			Timestamp: uint64(event.Timestamp()),
			Field:     convertAttributes(event.Attributes()),
		}

		logs = append(logs, log)
	}

	return logs
}

func getSpanKindString(kind ptrace.SpanKind) string {
	switch kind {
	case ptrace.SpanKindInternal:
		return "Internal"
	case ptrace.SpanKindServer:
		return "Server"
	case ptrace.SpanKindClient:
		return "Client"
	case ptrace.SpanKindProducer:
		return "Producer"
	case ptrace.SpanKindConsumer:
		return "Consumer"
	default:
		return "Unspecified"
	}
}

func getStatusCodeString(code ptrace.StatusCode) string {
	switch code {
	case ptrace.StatusCodeOk:
		return "Ok"
	case ptrace.StatusCodeError:
		return "Error"
	default:
		return "Unset"
	}
}

func (e *tracesExporter) sendToHydrolix(ctx context.Context, data []byte, table, transform string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-hdx-table", table)
	req.Header.Set("x-hdx-transform", transform)
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

	e.logger.Debug("successfully sent traces to Hydrolix",
		zap.String("table", table),
		zap.String("transform", transform))

	return nil
}
