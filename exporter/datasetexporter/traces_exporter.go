// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"fmt"
	"strings"

	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const ServiceNameKey = "service.name"

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Traces, error) {
	cfg := castConfig(config)
	e, err := newDatasetExporter("traces", cfg, set)
	if err != nil {
		return nil, fmt.Errorf("cannot get DataSetExporter: %w", err)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		config,
		e.consumeTraces,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithShutdown(e.shutdown),
	)
}

func buildEventFromSpan(
	bundle spanBundle,
	serverHost string,
	settings TracesSettings,
) *add_events.EventBundle {
	span := bundle.span
	resource := bundle.resource

	attrs := make(map[string]any)
	event := add_events.Event{
		Sev: int(plog.SeverityNumberInfo),
		Ts:  fmt.Sprintf("%d", span.StartTimestamp().AsTime().UnixNano()),
	}

	attrs["sca:schema"] = "tracing"
	attrs["sca:schemVer"] = 1
	attrs["sca:type"] = "span"

	attrs["name"] = span.Name()
	attrs["span_id"] = span.SpanID().String()
	if !span.ParentSpanID().IsEmpty() {
		attrs["parent_span_id"] = span.ParentSpanID().String()
	}
	attrs["trace_id"] = span.TraceID().String()

	attrs["start_time_unix_nano"] = fmt.Sprintf("%d", span.StartTimestamp().AsTime().UnixNano())
	attrs["end_time_unix_nano"] = fmt.Sprintf("%d", span.EndTimestamp().AsTime().UnixNano())
	attrs["duration_nano"] = fmt.Sprintf("%d", span.EndTimestamp().AsTime().UnixNano()-span.StartTimestamp().AsTime().UnixNano())

	attrs["kind"] = strings.ToLower(span.Kind().String())
	attrs["status_code"] = strings.ToLower(span.Status().Code().String())
	attrs["status_message"] = span.Status().Message()
	// for now we care only small subset of attributes
	// updateWithPrefixedValues(attrs, "resource_", "_", resource.Attributes().AsRaw(), 0)
	updateResource(attrs, resource.Attributes().AsRaw())

	// since attributes are overwriting existing keys, they have to be at the end
	updateWithPrefixedValues(attrs, "", settings.ExportSeparator, settings.ExportDistinguishingSuffix, span.Attributes().AsRaw(), 0)

	event.Attrs = attrs
	event.Log = "LT"
	event.Thread = "TT"
	event.ServerHost = inferServerHost(bundle.resource, attrs, serverHost)
	return &add_events.EventBundle{
		Event:  &event,
		Thread: &add_events.Thread{Id: "TT", Name: "traces"},
		Log:    &add_events.Log{Id: "LT", Attrs: map[string]any{}},
	}
}

const resourceName = "resource_name"
const resourceType = "resource_type"

type ResourceType string

const (
	Service = ResourceType("service")
	Process = ResourceType("process")
)

func updateResource(attrs map[string]any, resource map[string]any) {
	// first detect, whether there is key service.name
	// if it's there, we are done
	name, found := resource["service.name"]
	if found {
		attrs[resourceName] = name
		attrs[resourceType] = string(Service)
		return
	}

	// if we were not able to find service name, lets mark it as process
	attrs[resourceName] = ""
	attrs[resourceType] = string(Process)

	// but still try to search for anything, that start on service
	// if we found it, we will mark it as service
	for k, v := range resource {
		if strings.HasPrefix(k, "service") {
			attrs[resourceName] = ""
			attrs[resourceType] = string(Service)
			return
		}
		// when we find process.pid - lets use it as name
		if k == "process.pid" {
			attrs[resourceName] = v
		}
	}
}

type spanBundle struct {
	span     ptrace.Span
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
}

func buildEventsFromTraces(ld ptrace.Traces, serverHost string, settings TracesSettings) []*add_events.EventBundle {
	var spans = make([]spanBundle, 0)

	// convert spans into events
	resourceSpans := ld.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resource := resourceSpans.At(i).Resource()
		scopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scope := scopeSpans.At(j).Scope()
			spanRecords := scopeSpans.At(j).Spans()
			for k := 0; k < spanRecords.Len(); k++ {
				spanRecord := spanRecords.At(k)
				spans = append(spans, spanBundle{spanRecord, resource, scope})
			}
		}
	}

	events := make([]*add_events.EventBundle, len(spans))
	for i, span := range spans {
		events[i] = buildEventFromSpan(span, serverHost, settings)
	}

	return events
}

func (e *DatasetExporter) consumeTraces(_ context.Context, ld ptrace.Traces) error {
	return sendBatch(buildEventsFromTraces(ld, e.serverHost, e.exporterCfg.tracesSettings), e.client)
}
