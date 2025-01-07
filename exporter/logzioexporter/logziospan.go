// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"encoding/json"

	"github.com/jaegertracing/jaeger/model"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter/internal/dbmodel"
)

const (
	spanLogType = "jaegerSpan"
	// TagDotReplacementCharacter state which character should replace the dot in es
	tagDotReplacementCharacter = "@"
)

// logzioSpan is same as esSpan with a few different json field names and an addition on type field.
type logzioSpan struct {
	TraceID         dbmodel.TraceID     `json:"traceID"`
	OperationName   string              `json:"operationName,omitempty"`
	SpanID          dbmodel.SpanID      `json:"spanID"`
	References      []dbmodel.Reference `json:"references"`
	Flags           uint32              `json:"flags,omitempty"`
	StartTime       uint64              `json:"startTime"`
	StartTimeMillis uint64              `json:"startTimeMillis"`
	Timestamp       uint64              `json:"@timestamp"`
	Duration        uint64              `json:"duration"`
	Tags            []dbmodel.KeyValue  `json:"JaegerTags,omitempty"`
	Tag             map[string]any      `json:"JaegerTag,omitempty"`
	Logs            []dbmodel.Log       `json:"logs"`
	Process         dbmodel.Process     `json:"process,omitempty"`
	Type            string              `json:"type"`
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
func transformToLogzioSpanBytes(span *model.Span) ([]byte, error) {
	spanConverter := newFromDomain(true, getTagsValues(span.Tags), tagDotReplacementCharacter)
	jsonSpan := spanConverter.fromDomainEmbedProcess(span)
	newSpan := logzioSpan{
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
