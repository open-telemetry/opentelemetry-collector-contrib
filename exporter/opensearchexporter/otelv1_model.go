// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import "time"

type otelV1Span struct {
	TraceID                string            `json:"traceId"`
	SpanID                 string            `json:"spanId"`
	ParentSpanID           string            `json:"parentSpanId"`
	Name                   string            `json:"name"`
	Kind                   string            `json:"kind"`
	TraceState             string            `json:"traceState,omitempty"`
	StartTime              time.Time         `json:"startTime"`
	EndTime                time.Time         `json:"endTime"`
	Timestamp              time.Time         `json:"@timestamp"`
	Time                   time.Time         `json:"time"`
	DurationInNanos        int64             `json:"durationInNanos"`
	ServiceName            string            `json:"serviceName,omitempty"`
	Status                 otelV1SpanStatus  `json:"status"`
	TraceGroup             string            `json:"traceGroup,omitempty"`
	TraceGroupFields       *otelV1TraceGroup `json:"traceGroupFields,omitempty"`
	Events                 []otelV1SpanEvent `json:"events,omitempty"`
	Links                  []otelV1SpanLink  `json:"links,omitempty"`
	DroppedAttributesCount uint32            `json:"droppedAttributesCount"`
	DroppedEventsCount     uint32            `json:"droppedEventsCount"`
	DroppedLinksCount      uint32            `json:"droppedLinksCount"`
	Resource               otelV1Resource    `json:"resource"`
	InstrumentationScope   otelV1Scope       `json:"instrumentationScope"`
	Attributes             map[string]any    `json:"attributes,omitempty"`
}

type otelV1SpanStatus struct {
	Code    int32  `json:"code"`
	Message string `json:"message,omitempty"`
}

type otelV1TraceGroup struct {
	EndTime         time.Time `json:"endTime"`
	DurationInNanos int64     `json:"durationInNanos"`
	StatusCode      int32     `json:"statusCode"`
}

type otelV1SpanEvent struct {
	Name                   string         `json:"name"`
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	Time                   time.Time      `json:"time"`
}

type otelV1SpanLink struct {
	TraceID                string         `json:"traceId"`
	SpanID                 string         `json:"spanId"`
	TraceState             string         `json:"traceState,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
}

type otelV1Resource struct {
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	SchemaURL              string         `json:"schemaUrl,omitempty"`
}

type otelV1Scope struct {
	Name                   string         `json:"name,omitempty"`
	Version                string         `json:"version,omitempty"`
	SchemaURL              string         `json:"schemaUrl,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
}

type otelV1LogRecord struct {
	Timestamp              time.Time      `json:"@timestamp"`
	Time                   time.Time      `json:"time"`
	ObservedTime           time.Time      `json:"observedTime"`
	Severity               otelV1Severity `json:"severity"`
	Body                   string         `json:"body,omitempty"`
	EventName              string         `json:"eventName,omitempty"`
	TraceID                string         `json:"traceId,omitempty"`
	SpanID                 string         `json:"spanId,omitempty"`
	Flags                  int64          `json:"flags"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	Resource               otelV1Resource `json:"resource"`
	InstrumentationScope   otelV1Scope    `json:"instrumentationScope"`
	Attributes             map[string]any `json:"attributes,omitempty"`
}

type otelV1Severity struct {
	Number int32  `json:"number"`
	Text   string `json:"text,omitempty"`
}
