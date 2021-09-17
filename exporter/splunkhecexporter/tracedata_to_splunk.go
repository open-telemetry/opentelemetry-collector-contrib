// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// hecEvent is a data structure holding a span event to export explicitly to Splunk HEC.
type hecEvent struct {
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Name       string                 `json:"name"`
	Timestamp  pdata.Timestamp        `json:"timestamp"`
}

// hecLink is a data structure holding a span link to export explicitly to Splunk HEC.
type hecLink struct {
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	TraceState pdata.TraceState       `json:"trace_state"`
}

// hecSpanStatus is a data structure holding the status of a span to export explicitly to Splunk HEC.
type hecSpanStatus struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// hecSpan is a data structure used to export explicitly a span to Splunk HEC.
type hecSpan struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	ParentSpan string                 `json:"parent_span_id"`
	Name       string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	EndTime    pdata.Timestamp        `json:"end_time"`
	Kind       string                 `json:"kind"`
	Status     hecSpanStatus          `json:"status,omitempty"`
	StartTime  pdata.Timestamp        `json:"start_time"`
	Events     []hecEvent             `json:"events,omitempty"`
	Links      []hecLink              `json:"links,omitempty"`
}

func traceDataToSplunk(logger *zap.Logger, data pdata.Traces, config *Config) ([]*splunk.Event, int) {
	sourceKey := config.HecToOtelAttrs.Source
	sourceTypeKey := config.HecToOtelAttrs.SourceType
	indexKey := config.HecToOtelAttrs.Index
	hostKey := config.HecToOtelAttrs.Host

	numDroppedSpans := 0
	splunkEvents := make([]*splunk.Event, 0, data.SpanCount())
	rss := data.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		host := unknownHostName
		source := config.Source
		sourceType := config.SourceType
		index := config.Index
		commonFields := map[string]interface{}{}
		rs.Resource().Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			switch k {
			case hostKey:
				host = v.StringVal()
				commonFields[conventions.AttributeHostName] = host
			case sourceKey:
				source = v.StringVal()
			case sourceTypeKey:
				sourceType = v.StringVal()
			case indexKey:
				index = v.StringVal()
			default:
				commonFields[k] = v.AsString()
			}
			return true
		})
		ilss := rs.InstrumentationLibrarySpans()
		for sils := 0; sils < ilss.Len(); sils++ {
			ils := ilss.At(sils)
			spans := ils.Spans()
			for si := 0; si < spans.Len(); si++ {
				span := spans.At(si)
				se := &splunk.Event{
					Time:       timestampToSecondsWithMillisecondPrecision(span.StartTimestamp()),
					Host:       host,
					Source:     source,
					SourceType: sourceType,
					Index:      index,
					Event:      toHecSpan(logger, span),
					Fields:     commonFields,
				}
				splunkEvents = append(splunkEvents, se)
			}
		}
	}

	return splunkEvents, numDroppedSpans
}

func toHecSpan(logger *zap.Logger, span pdata.Span) hecSpan {
	attributes := map[string]interface{}{}
	span.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		attributes[k] = convertAttributeValue(v, logger)
		return true
	})

	links := make([]hecLink, span.Links().Len())
	for i := 0; i < span.Links().Len(); i++ {
		link := span.Links().At(i)
		linkAttributes := map[string]interface{}{}
		link.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			linkAttributes[k] = convertAttributeValue(v, logger)
			return true
		})
		links[i] = hecLink{
			Attributes: linkAttributes,
			TraceID:    link.TraceID().HexString(),
			SpanID:     link.SpanID().HexString(),
			TraceState: link.TraceState(),
		}
	}
	events := make([]hecEvent, span.Events().Len())
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		eventAttributes := map[string]interface{}{}
		event.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			eventAttributes[k] = convertAttributeValue(v, logger)
			return true
		})
		events[i] = hecEvent{
			Attributes: eventAttributes,
			Name:       event.Name(),
			Timestamp:  event.Timestamp(),
		}
	}
	status := hecSpanStatus{
		Message: span.Status().Message(),
		Code:    span.Status().Code().String(),
	}
	return hecSpan{
		TraceID:    span.TraceID().HexString(),
		SpanID:     span.SpanID().HexString(),
		ParentSpan: span.ParentSpanID().HexString(),
		Name:       span.Name(),
		Attributes: attributes,
		StartTime:  span.StartTimestamp(),
		EndTime:    span.EndTimestamp(),
		Kind:       span.Kind().String(),
		Status:     status,
		Links:      links,
		Events:     events,
	}
}
