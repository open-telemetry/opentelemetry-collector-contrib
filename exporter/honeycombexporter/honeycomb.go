// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package honeycombexporter

import (
	"context"
	"time"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	libhoney "github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

// User agent string to use when sending events to Honeycomb.
const (
	oTelCollectorUserAgentStr = "Honeycomb-OpenTelemetry-Collector"
)

// honeycombExporter is the object that sends events to honeycomb.
type honeycombExporter struct {
	builder *libhoney.Builder
	onError func(error)
	logger  *zap.Logger
}

// event represents a honeycomb event.
type event struct {
	ID              string  `json:"trace.span_id"`
	TraceID         string  `json:"trace.trace_id"`
	ParentID        string  `json:"trace.parent_id,omitempty"`
	Name            string  `json:"name"`
	Status          string  `json:"response.status_code,omitempty"`
	HasRemoteParent bool    `json:"has_remote_parent"`
	DurationMilli   float64 `json:"duration_ms"`
}

// spanEvent represents an event attached to a specific span.¬
type spanEvent struct {
	Name       string `json:"name"`
	TraceID    string `json:"trace.trace_id"`
	ParentID   string `json:"trace.parent_id,omitempty"`
	ParentName string `json:"trace.parent_name,omitempty"`
	SpanType   string `json:"meta.span_type"`
}

// link represents a link to a trace and span that lives elsewhere.
// TraceID and ParentID are used to identify the span with which the trace is associated
// We are modeling Links for now as child spans rather than properties of the event.
type link struct {
	TraceID     string      `json:"trace.trace_id"`
	ParentID    string      `json:"trace.parent_id,omitempty"`
	LinkTraceID string      `json:"trace.link.trace_id"`
	LinkSpanID  string      `json:"trace.link.span_id"`
	SpanType    string      `json:"meta.span_type"`
	RefType     spanRefType `json:"ref_type,omitempty"`
}

// spanRefType defines the relationship a Link has to a trace or a span. It can
// either be a child of, or a follows from relationship.
type spanRefType int64

// newHoneycombTraceExporter creates and returns a new honeycombExporter. It
// wraps the exporter in the component.TraceExporterOld helper method.
func newHoneycombTraceExporter(cfg *Config, logger *zap.Logger) (component.TraceExporterOld, error) {
	libhoneyConfig := libhoney.Config{
		WriteKey:   cfg.APIKey,
		Dataset:    cfg.Dataset,
		APIHost:    cfg.APIURL,
		SampleRate: cfg.SampleRate,
	}
	userAgent := oTelCollectorUserAgentStr
	libhoney.UserAgentAddition = userAgent

	if cfg.Debug {
		libhoneyConfig.Logger = &libhoney.DefaultLogger{}
	}

	if err := libhoney.Init(libhoneyConfig); err != nil {
		return nil, err
	}
	builder := libhoney.NewBuilder()
	exporter := &honeycombExporter{
		builder: builder,
		logger:  logger,
		onError: func(err error) {
			logger.Warn(err.Error())
		},
	}

	return exporterhelper.NewTraceExporterOld(
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithShutdown(exporter.Shutdown))
}

// pushTraceData is the method called when trace data is available. It will be
// responsible for sending a batch of events.
func (e *honeycombExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	// Run the error logger. This just listens for messages in the error
	// response queue and writes them out using the logger.
	ctx, cancel := context.WithCancel(ctx)
	go e.RunErrorLogger(ctx, libhoney.TxResponses())
	defer cancel()

	// Extract Node and Resource attributes, labels and other information.
	// Because these exist on the TraceData, they will be added to every span.
	traceLevelFields := getTraceLevelFields(td)
	addTraceLevelFields := func(ev *libhoney.Event) {
		for k, v := range traceLevelFields {
			ev.AddField(k, v)
		}
	}

	for _, span := range td.Spans {
		ev := e.builder.NewEvent()
		addTraceLevelFields(ev)

		// Treat resource labels as underlays, with any same-keyed span attributes taking
		// precedence. Apply them first.
		if span.Resource != nil && span.Resource.Labels != nil {
			resourceType := span.Resource.GetType()
			if len(resourceType) != 0 {
				ev.AddField("resource_type", resourceType)
			}
			for k, v := range span.Resource.Labels {
				ev.AddField(k, v)
			}
		}

		if len(span.GetParentSpanId()) == 0 || hasRemoteParent(span) {
			if td.Node != nil {
				for k, v := range td.Node.Attributes {
					ev.AddField(k, v)
				}
			}
		}

		if attrs := spanAttributesToMap(span.GetAttributes()); attrs != nil {
			for k, v := range attrs {
				ev.AddField(k, v)
			}
		}

		ev.Timestamp = timestampToTime(span.GetStartTime())
		startTime := timestampToTime(span.GetStartTime())
		endTime := timestampToTime(span.GetEndTime())

		ev.Add(event{
			ID:              getHoneycombSpanID(span.GetSpanId()),
			TraceID:         getHoneycombTraceID(span.GetTraceId()),
			ParentID:        getHoneycombSpanID(span.GetParentSpanId()),
			Name:            truncatableStringAsString(span.GetName()),
			DurationMilli:   float64(endTime.Sub(startTime)) / float64(time.Millisecond),
			HasRemoteParent: hasRemoteParent(span),
		})

		e.sendMessageEvents(td, span, traceLevelFields)
		e.sendSpanLinks(span)

		ev.AddField("status.code", getStatusCode(span.Status))
		ev.AddField("status.message", getStatusMessage(span.Status))
		ev.AddField("has_remote_parent", !span.GetSameProcessAsParentSpan().GetValue())
		ev.AddField("child_span_count", span.GetChildSpanCount())

		if err := ev.SendPresampled(); err != nil {
			errs = append(errs, err)
		} else {
			goodSpans++
		}
	}

	return len(td.Spans) - goodSpans, componenterror.CombineErrors(errs)
}

// sendSpanLinks gets the list of links associated with this span and sends them as
// separate events to Honeycomb, with a span type "link".
func (e *honeycombExporter) sendSpanLinks(span *tracepb.Span) {
	links := span.GetLinks()

	if links == nil {
		return
	}

	for _, l := range links.GetLink() {
		ev := e.builder.NewEvent()
		ev.Add(link{
			TraceID:     getHoneycombTraceID(span.GetTraceId()),
			ParentID:    getHoneycombSpanID(span.GetSpanId()),
			LinkTraceID: getHoneycombTraceID(l.GetTraceId()),
			LinkSpanID:  getHoneycombSpanID(l.GetSpanId()),
			SpanType:    "link",
			RefType:     spanRefType(l.GetType()),
		})
		attrs := spanAttributesToMap(l.GetAttributes())
		for k, v := range attrs {
			ev.AddField(k, v)
		}
		if err := ev.SendPresampled(); err != nil {
			e.onError(err)
		}
	}
}

// sendMessageEvents gets the list of timeevents from the span and sends them as
// separate events to Honeycomb, with a span type "span_event".
func (e *honeycombExporter) sendMessageEvents(td consumerdata.TraceData, span *tracepb.Span, traceFields map[string]interface{}) {
	timeEvents := span.GetTimeEvents()
	if timeEvents == nil {
		return
	}

	for _, event := range timeEvents.TimeEvent {
		annotation := event.GetAnnotation()
		if annotation == nil {
			continue
		}

		ts := timestampToTime(event.GetTime())
		name := annotation.GetDescription().GetValue()
		attrs := spanAttributesToMap(annotation.GetAttributes())

		// treat trace level fields as underlays with same keyed span attributes taking precedence.
		ev := e.builder.NewEvent()
		for k, v := range traceFields {
			ev.AddField(k, v)
		}
		if span.Resource != nil && span.Resource.Labels != nil {
			resourceType := span.Resource.GetType()
			if len(resourceType) != 0 {
				ev.AddField("resource_type", resourceType)
			}
			for k, v := range span.Resource.Labels {
				ev.AddField(k, v)
			}
		}
		if len(span.GetParentSpanId()) == 0 || hasRemoteParent(span) {
			if td.Node != nil {
				for k, v := range td.Node.Attributes {
					ev.AddField(k, v)
				}
			}
		}
		for k, v := range attrs {
			ev.AddField(k, v)
		}
		ev.Timestamp = ts
		ev.Add(spanEvent{
			Name:       name,
			TraceID:    getHoneycombTraceID(span.GetTraceId()),
			ParentID:   getHoneycombSpanID(span.GetSpanId()),
			ParentName: truncatableStringAsString(span.GetName()),
			SpanType:   "span_event",
		})
		if err := ev.SendPresampled(); err != nil {
			e.onError(err)
		}
	}
}

// Shutdown takes care of any cleanup tasks that need to be carried out. In
// this case, we close the honeycomb sdk which flushes any events still in the
// queue and closes any open channels between queues.
func (e *honeycombExporter) Shutdown(context.Context) error {
	libhoney.Close()
	return nil
}

// RunErrorLogger consumes from the response queue, calling the onError callback
// when errors are encountered.
//
// This method will block until the passed context.Context is canceled, or until
// exporter.Close is called.
func (e *honeycombExporter) RunErrorLogger(ctx context.Context, responses chan transmission.Response) {
	for {
		select {
		case r, ok := <-responses:
			if !ok {
				return
			}
			if r.Err != nil {
				e.onError(r.Err)
			}
		case <-ctx.Done():
			return
		}
	}
}
