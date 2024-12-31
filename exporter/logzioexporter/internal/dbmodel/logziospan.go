// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbmodel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"encoding/json"

	"github.com/jaegertracing/jaeger/model"
)

const (
	spanLogType = "jaegerSpan"
	// TagDotReplacementCharacter state which character should replace the dot in es
	tagDotReplacementCharacter = "@"
)

// LogzioSpan is same as esSpan with a few different json field names and an addition on type field.
type LogzioSpan struct {
	TraceID         TraceID        `json:"traceID"`
	OperationName   string         `json:"operationName,omitempty"`
	SpanID          SpanID         `json:"spanID"`
	References      []Reference    `json:"references"`
	Flags           uint32         `json:"flags,omitempty"`
	StartTime       uint64         `json:"startTime"`
	StartTimeMillis uint64         `json:"startTimeMillis"`
	Timestamp       uint64         `json:"@timestamp"`
	Duration        uint64         `json:"duration"`
	Tags            []KeyValue     `json:"JaegerTags,omitempty"`
	Tag             map[string]any `json:"JaegerTag,omitempty"`
	Logs            []Log          `json:"logs"`
	Process         Process        `json:"process,omitempty"`
	Type            string         `json:"type"`
}

func getTagsValues(tags []model.KeyValue) []string {
	values := make([]string, len(tags))
	for i := range tags {
		values[i] = tags[i].VStr
	}
	return values
}

// transformToLogzioSpanBytes receives a Jaeger span, converts it to logzio span and returns it as a byte array.
// The main differences between Jaeger span and logzio span are arrays which are represented as maps
func TransformToLogzioSpanBytes(span *model.Span) ([]byte, error) {
	spanConverter := newFromDomain(true, getTagsValues(span.Tags), tagDotReplacementCharacter)
	jsonSpan := spanConverter.fromDomainEmbedProcess(span)
	newSpan := LogzioSpan{
		TraceID:         jsonSpan.TraceID,
		OperationName:   jsonSpan.OperationName,
		SpanID:          jsonSpan.SpanID,
		References:      jsonSpan.References,
		Flags:           jsonSpan.Flags,
		StartTime:       jsonSpan.StartTime,
		StartTimeMillis: jsonSpan.StartTimeMillis,
		Timestamp:       jsonSpan.StartTimeMillis,
		Duration:        jsonSpan.Duration,
		Tags:            jsonSpan.Tags,
		Tag:             jsonSpan.Tag,
		Process:         jsonSpan.Process,
		Logs:            jsonSpan.Logs,
		Type:            spanLogType,
	}
	return json.Marshal(newSpan)
}
