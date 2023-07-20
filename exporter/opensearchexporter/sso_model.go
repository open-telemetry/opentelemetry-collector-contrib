// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"time"
)

type DataStream struct {
	Dataset   string `json:"dataset,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Type      string `json:"type,omitempty"`
}

type SSOSpanEvent struct {
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	Name                   string         `json:"name"`
	ObservedTimestamp      *time.Time     `json:"observedTimestamp,omitempty"`
	Timestamp              *time.Time     `json:"@timestamp,omitempty"`
}

type SSOSpanLinks struct {
	Attributes             map[string]any `json:"attributes,omitempty"`
	SpanID                 string         `json:"spanId,omitempty"`
	TraceID                string         `json:"traceId,omitempty"`
	TraceState             string         `json:"traceState,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount,omitempty"`
}

type SSOSpan struct {
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	DroppedEventsCount     uint32         `json:"droppedEventsCount"`
	DroppedLinksCount      uint32         `json:"droppedLinksCount"`
	EndTime                time.Time      `json:"endTime"`
	Events                 []SSOSpanEvent `json:"events,omitempty"`
	InstrumentationScope   struct {
		Attributes             map[string]any `json:"attributes,omitempty"`
		DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
		Name                   string         `json:"name"`
		SchemaURL              string         `json:"schemaUrl"`
		Version                string         `json:"version"`
	} `json:"instrumentationScope,omitempty"`
	Kind         string         `json:"kind"`
	Links        []SSOSpanLinks `json:"links,omitempty"`
	Name         string         `json:"name"`
	ParentSpanID string         `json:"parentSpanId"`
	Resource     map[string]any `json:"resource,omitempty"`
	SpanID       string         `json:"spanId"`
	StartTime    time.Time      `json:"startTime"`
	Status       struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	TraceID    string `json:"traceId"`
	TraceState string `json:"traceState"`
}
