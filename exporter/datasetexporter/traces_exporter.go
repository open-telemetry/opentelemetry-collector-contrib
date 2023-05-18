// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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
	e, err := newDatasetExporter("logs", cfg, set.Logger)
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
		exporterhelper.WithShutdown(e.shutdown),
	)
}

func buildEventFromSpan(
	bundle spanBundle,
	tracker *spanTracker,
) *add_events.EventBundle {
	span := bundle.span
	resource := bundle.resource

	attrs := make(map[string]interface{})
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

	service, serviceFound := resource.Attributes().Get(ServiceNameKey)
	if serviceFound {
		attrs["services"] = service.AsString()
	}

	if tracker != nil {
		// find tracking info
		if !span.TraceID().IsEmpty() {
			key := newTraceAndSpan(span.TraceID(), span.SpanID())

			// find previous information
			info, ok := tracker.spans[key]
			if ok {
				// don't count itself
				attrs["span_count"] = info.spanCount
				attrs["error_count"] = info.errorCount
				updateServices(attrs, info.services)

				delete(tracker.spans, key)
			}

			// propagate those values to the parent
			pKey := newTraceAndSpan(span.TraceID(), span.ParentSpanID())
			if !span.ParentSpanID().IsEmpty() {
				// find previous information
				pInfo, pOk := tracker.spans[pKey]
				if pOk {
					// so we know that this span is parent of something
					pInfo.spanCount += info.spanCount
					pInfo.errorCount += info.errorCount
					for k, v := range info.services {
						pInfo.services[k] = v
					}
					tracker.spans[pKey] = pInfo
				} else {
					attrs["missing_parent"] = 1
				}
			} else {
				// we have processed parent, so lets remove it
				delete(tracker.spans, pKey)
			}
		}
	}

	// since attributes are overwriting existing keys, they have to be at the end
	// updateWithPrefixedValues(attrs, "attributes_", "_", span.Attributes().AsRaw(), 0)
	updateWithPrefixedValues(attrs, "", "_", span.Attributes().AsRaw(), 0)

	event.Attrs = attrs
	event.Log = "LT"
	event.Thread = "TT"
	return &add_events.EventBundle{
		Event:  &event,
		Thread: &add_events.Thread{Id: "TT", Name: "traces"},
		Log:    &add_events.Log{Id: "LT", Attrs: map[string]interface{}{}},
	}
}

const resourceName = "resource_name"
const resourceType = "resource_type"

type ResourceType string

const (
	Service = ResourceType("service")
	Process = ResourceType("process")
)

func updateResource(attrs map[string]interface{}, resource map[string]any) {
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

func updateServices(attrs map[string]interface{}, services map[string]bool) {
	servicesA := make([]string, 0)
	service, serviceFound := attrs["services"]
	if serviceFound {
		delete(services, service.(string))
	}

	if len(services) > 0 {
		for k := range services {
			servicesA = append(servicesA, k)
		}

		sort.Strings(servicesA)
		attrs["services"] = fmt.Sprintf("%s,%s", attrs["services"], strings.Join(servicesA, ","))
	}
}

type spanBundle struct {
	span     ptrace.Span
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
}

func buildEventsFromTraces(ld ptrace.Traces, tracker *spanTracker) []*add_events.EventBundle {
	var events []*add_events.EventBundle
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

	if tracker != nil {
		// sort by end time
		// there is no guarantee that parent ends last, so lets place at least all root nodes
		// at the end
		// to get even better results, we should do topological sorting, but still events
		// can be in multiple batches, so it will not be perfect either
		// this should be implemented on the backend to support multiple collectors anyway
		sort.Slice(spans, func(i, j int) bool {
			if spans[i].span.ParentSpanID().IsEmpty() == spans[j].span.ParentSpanID().IsEmpty() {
				return spans[i].span.EndTimestamp() < spans[j].span.EndTimestamp()
			}

			return !spans[i].span.ParentSpanID().IsEmpty()
		})
	}

	for _, span := range spans {
		if tracker != nil {
			tracker.update(span)
		}
		events = append(events, buildEventFromSpan(span, tracker))
	}

	if tracker != nil {
		// purge old values
		tracker.purge()
	}

	return events
}

func (e *DatasetExporter) consumeTraces(ctx context.Context, ld ptrace.Traces) error {

	return sendBatch(buildEventsFromTraces(ld, e.spanTracker), e.client)
}

type spanInfo struct {
	errorCount int
	spanCount  int
	createdAt  time.Time
	services   map[string]bool
}

type TraceAndSpan [24]byte

func newTraceAndSpan(traceID pcommon.TraceID, spanID pcommon.SpanID) TraceAndSpan {
	var traceAndSpan TraceAndSpan
	copy(traceAndSpan[:], traceID[:])
	copy(traceAndSpan[16:], spanID[:])
	return traceAndSpan
}

func (ts TraceAndSpan) split() (traceID pcommon.TraceID, spanID pcommon.SpanID) {
	copy(traceID[:], ts[:16])
	copy(spanID[:], ts[16:])
	return traceID, spanID
}

func (ts TraceAndSpan) String() string {
	traceID, spanID := ts.split()
	return traceID.String() + spanID.String()
}

type spanTracker struct {
	spans      map[TraceAndSpan]spanInfo
	purgedAt   time.Time
	purgeEvery time.Duration
}

func newSpanTracker(purgeEvery time.Duration) *spanTracker {
	return &spanTracker{
		spans:      make(map[TraceAndSpan]spanInfo),
		purgedAt:   time.Now(),
		purgeEvery: purgeEvery,
	}
}

func (tt *spanTracker) update(bundle spanBundle) {
	span := bundle.span
	resource := bundle.resource
	// for empty trace ids do nothing
	if span.TraceID().IsEmpty() {
		return
	}

	key := newTraceAndSpan(span.TraceID(), span.ParentSpanID())

	// find previous information
	info, ok := tt.spans[key]
	if !ok {
		info = spanInfo{
			createdAt: time.Now(),
			services:  make(map[string]bool),
		}
	}

	// increase counters
	info.spanCount++
	if span.Status().Code() == ptrace.StatusCodeError {
		info.errorCount++
	}

	// update services
	service, serviceFound := resource.Attributes().Get(ServiceNameKey)
	if serviceFound {
		info.services[service.AsString()] = true
	}

	tt.spans[key] = info
}

func (tt *spanTracker) purge() int {
	// if it was purged recently, do nothing
	if time.Since(tt.purgedAt) < tt.purgeEvery {
		return 0
	}

	// find and purge old values
	purged := 0
	for key, info := range tt.spans {
		if time.Since(info.createdAt) > tt.purgeEvery {
			purged++
			delete(tt.spans, key)
		}
	}

	tt.purgedAt = time.Now()
	return purged
}
