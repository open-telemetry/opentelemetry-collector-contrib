// Copyright The OpenTelemetry Authors
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

package datasetexporter

import (
	"context"
	"fmt"
	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Traces, error) {
	cfg := castConfig(config)
	e, err := getDatasetExporter("logs", cfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot get DataSetExpoter: %w", err)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		config,
		e.consumeTraces,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithShutdown(func(context.Context) error {
			e.shutdown()
			return nil
		}),
	)
}

func buildEventFromSpan(span ptrace.Span, resource pcommon.Resource, scope pcommon.InstrumentationScope) *add_events.EventBundle {
	var attrs = make(map[string]interface{})
	var event = add_events.Event{
		Sev: int(plog.SeverityNumberInfo),
		Ts:  fmt.Sprintf("%d", span.StartTimestamp().AsTime().UnixNano()),
	}
	attrs["OTEL_TYPE"] = "trace"
	attrs["sca:schema"] = "tracing"
	attrs["sca:schemVer"] = 1
	attrs["sca:type"] = "span"

	attrs["name"] = span.Name()
	if len(span.Name()) > 0 {
		attrs["message"] = fmt.Sprintf(
			"OtelExporter - Span - %s: %s",
			span.Name(),
			span.SpanID().String(),
		)
	}

	attrs["span_id"] = span.SpanID().String()
	attrs["parent_span_id"] = span.ParentSpanID().String()
	attrs["trace_id"] = span.TraceID().String()

	attrs["start_time_unix_nano"] = span.StartTimestamp().AsTime().UnixNano()
	attrs["end_time_unix_nano"] = span.EndTimestamp().AsTime().UnixNano()
	attrs["duration_nano"] = span.EndTimestamp().AsTime().UnixNano() - span.StartTimestamp().AsTime().UnixNano()

	attrs["kind"] = span.Kind().String()
	attrs["status_code"] = span.Status().Code().String()
	attrs["status_message"] = span.Status().Message()

	updateWithPrefixedValues(attrs, "resource_", "_", resource.Attributes().AsRaw(), 0)
	updateWithPrefixedValues(attrs, "attributes_", "_", span.Attributes().AsRaw(), 0)

	/*
		// we do not care for now about these properties
		attrs["trace_state"] = span.TraceState().AsRaw()
		attrs["dropped_attributes_count"] = span.DroppedAttributesCount()
		attrs["dropped_events_count"] = span.DroppedEventsCount()
		attrs["dropped_links_count"] = span.DroppedLinksCount()
		for i := 0; i < span.Events().Len(); i++ {
			attrs[fmt.Sprintf("event.%d.index", i)] = i
			event := span.Events().At(i)
			attrs[fmt.Sprintf("event.%d.name", i)] = event.Name()
			attrs[fmt.Sprintf("event.%d.timestamp", i)] = event.Timestamp()
			attrs[fmt.Sprintf("event.%d.timestamp.ns", i)] = event.Timestamp().AsTime().UnixNano()
			updateWithPrefixedValues(attrs, fmt.Sprintf("event.%d.attributes.", i), ".", event.Attributes().AsRaw(), 0)
		}

		for i := 0; i < span.Links().Len(); i++ {
			attrs[fmt.Sprintf("link.%d.index", i)] = i
			link := span.Links().At(i)
			attrs[fmt.Sprintf("link.%d.span_id", i)] = link.SpanID().String()
			attrs[fmt.Sprintf("link.%d.trace_id", i)] = link.TraceID().String()
			attrs[fmt.Sprintf("link.%d.trace_state", i)] = link.TraceState()
			updateWithPrefixedValues(attrs, fmt.Sprintf("link.%d.attributes.", i), ".", link.Attributes().AsRaw(), 0)
		}

		attrs["scope.name"] = scope.Name()
		updateWithPrefixedValues(attrs, "scope.attributes.", ".", scope.Attributes().AsRaw(), 0)
	*/
	event.Attrs = attrs
	event.Log = "LT"
	event.Thread = "TT"
	return &add_events.EventBundle{
		Event:  &event,
		Thread: &add_events.Thread{Id: "TT", Name: "traces"},
		Log:    &add_events.Log{Id: "LT", Attrs: map[string]interface{}{}},
	}

}

func (e *datasetExporter) consumeTraces(ctx context.Context, ld ptrace.Traces) error {
	var events []*add_events.EventBundle

	resourceLogs := ld.ResourceSpans()
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i).Resource()
		scopeLogs := resourceLogs.At(i).ScopeSpans()
		for j := 0; j < scopeLogs.Len(); j++ {
			scope := scopeLogs.At(j).Scope()
			logRecords := scopeLogs.At(j).Spans()
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				events = append(events, buildEventFromSpan(logRecord, resource, scope))
			}
		}
	}
	return sendBatch(events, e.client)
}
