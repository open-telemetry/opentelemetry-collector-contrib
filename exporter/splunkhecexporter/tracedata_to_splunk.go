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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// HecEvent is a data structure holding a span event to export explicitly to Splunk HEC.
type HecEvent struct {
	Attributes map[string]interface{}  `json:"attributes,omitempty"`
	Name       string                  `json:"name"`
	Timestamp  pdata.TimestampUnixNano `json:"timestamp"`
}

// HecLink is a data structure holding a span link to export explicitly to Splunk HEC.
type HecLink struct {
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	TraceState pdata.TraceState       `json:"trace_state"`
}

// HecSpanStatus is a data structure holding the status of a span to export explicitly to Splunk HEC.
type HecSpanStatus struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// HecSpan is a data structure used to export explicitly a span to Splunk HEC.
type HecSpan struct {
	TraceID    string                  `json:"trace_id"`
	SpanID     string                  `json:"span_id"`
	ParentSpan string                  `json:"parent_span_id"`
	Name       string                  `json:"name"`
	Attributes map[string]interface{}  `json:"attributes,omitempty"`
	EndTime    pdata.TimestampUnixNano `json:"end_time"`
	Kind       string                  `json:"kind"`
	Status     HecSpanStatus           `json:"status,omitempty"`
	StartTime  pdata.TimestampUnixNano `json:"start_time"`
	Events     []HecEvent              `json:"events,omitempty"`
	Links      []HecLink               `json:"links,omitempty"`
}

func traceDataToSplunk(logger *zap.Logger, data pdata.Traces, config *Config) ([]*splunk.Event, int) {
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
		resource := rs.Resource()
		attributes := resource.Attributes()
		if conventionHost, isSet := attributes.Get(conventions.AttributeHostName); isSet {
			host = conventionHost.StringVal()
		}
		if sourceSet, isSet := attributes.Get(conventions.AttributeServiceName); isSet {
			source = sourceSet.StringVal()
		}
		if sourcetypeSet, isSet := attributes.Get(splunk.SourcetypeLabel); isSet {
			sourceType = sourcetypeSet.StringVal()
		}
		if indexSet, isSet := attributes.Get(splunk.IndexLabel); isSet {
			index = indexSet.StringVal()
		}
		attributes.ForEach(func(k string, v pdata.AttributeValue) {
			commonFields[k] = tracetranslator.AttributeValueToString(v, false)
		})

		if sourceSet, isSet := rs.Resource().Attributes().Get(conventions.AttributeServiceName); isSet {
			source = sourceSet.StringVal()
		}
		if sourcetypeSet, isSet := rs.Resource().Attributes().Get(splunk.SourcetypeLabel); isSet {
			sourceType = sourcetypeSet.StringVal()
		}
		rs.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			commonFields[k] = tracetranslator.AttributeValueToString(v, false)
		})
		ilss := rs.InstrumentationLibrarySpans()
		for sils := 0; sils < ilss.Len(); sils++ {
			ils := ilss.At(sils)
			spans := ils.Spans()
			for si := 0; si < spans.Len(); si++ {
				span := spans.At(si)
				se := &splunk.Event{
					Time:       timestampToSecondsWithMillisecondPrecision(span.StartTime()),
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

func toHecSpan(logger *zap.Logger, span pdata.Span) HecSpan {
	attributes := map[string]interface{}{}
	span.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		attributes[k] = convertAttributeValue(v, logger)
	})

	links := make([]HecLink, span.Links().Len())
	for i := 0; i < span.Links().Len(); i++ {
		link := span.Links().At(i)
		linkAttributes := map[string]interface{}{}
		link.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			linkAttributes[k] = convertAttributeValue(v, logger)
		})
		links[i] = HecLink{
			Attributes: linkAttributes,
			TraceID:    link.TraceID().HexString(),
			SpanID:     link.SpanID().HexString(),
			TraceState: link.TraceState(),
		}
	}
	events := make([]HecEvent, span.Events().Len())
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		eventAttributes := map[string]interface{}{}
		event.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			eventAttributes[k] = convertAttributeValue(v, logger)
		})
		events[i] = HecEvent{
			Attributes: eventAttributes,
			Name:       event.Name(),
			Timestamp:  event.Timestamp(),
		}
	}
	status := HecSpanStatus{
		Message: span.Status().Message(),
		Code:    span.Status().Code().String(),
	}
	return HecSpan{
		TraceID:    span.TraceID().HexString(),
		SpanID:     span.SpanID().HexString(),
		ParentSpan: span.ParentSpanID().HexString(),
		Name:       span.Name(),
		Attributes: attributes,
		StartTime:  span.StartTime(),
		EndTime:    span.EndTime(),
		Kind:       span.Kind().String(),
		Status:     status,
		Links:      links,
		Events:     events,
	}
}
