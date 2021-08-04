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

	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// User agent string to use when sending events to Honeycomb.
const (
	oTelCollectorUserAgentStr = "Honeycomb-OpenTelemetry-Collector"
)

// honeycombExporter is the object that sends events to honeycomb.
type honeycombExporter struct {
	builder             *libhoney.Builder
	onError             func(error)
	logger              *zap.Logger
	sampleRateAttribute string
}

// event represents a honeycomb event.
type event struct {
	ID            string  `json:"trace.span_id"`
	TraceID       string  `json:"trace.trace_id"`
	ParentID      string  `json:"trace.parent_id,omitempty"`
	Name          string  `json:"name"`
	Status        string  `json:"response.status_code,omitempty"`
	DurationMilli float64 `json:"duration_ms"`
}

// spanEvent represents an event attached to a specific span.¬
type spanEvent struct {
	Name           string `json:"name"`
	TraceID        string `json:"trace.trace_id"`
	ParentID       string `json:"trace.parent_id,omitempty"`
	ParentName     string `json:"trace.parent_name,omitempty"`
	AnnotationType string `json:"meta.annotation_type"`
}

// link represents a link to a trace and span that lives elsewhere.
// TraceID and ParentID are used to identify the span with which the trace is associated
// We are modeling Links for now as child spans rather than properties of the event.
type link struct {
	TraceID        string `json:"trace.trace_id"`
	ParentID       string `json:"trace.parent_id,omitempty"`
	LinkTraceID    string `json:"trace.link.trace_id"`
	LinkSpanID     string `json:"trace.link.span_id"`
	AnnotationType string `json:"meta.annotation_type"`
}

// newHoneycombTracesExporter creates and returns a new honeycombExporter. It
// wraps the exporter in the component.TracesExporterOld helper method.
func newHoneycombTracesExporter(cfg *Config, logger *zap.Logger) (*honeycombExporter, error) {
	libhoneyConfig := libhoney.Config{
		WriteKey: cfg.APIKey,
		Dataset:  cfg.Dataset,
		APIHost:  cfg.APIURL,
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
		sampleRateAttribute: cfg.SampleRateAttribute,
	}

	return exporter, nil
}

// pushTraceData is the method called when trace data is available. It will be
// responsible for sending a batch of events.
func (e *honeycombExporter) pushTraceData(ctx context.Context, td pdata.Traces) error {
	var errs []error

	// Run the error logger. This just listens for messages in the error
	// response queue and writes them out using the logger.
	ctx, cancel := context.WithCancel(ctx)
	go e.RunErrorLogger(ctx, libhoney.TxResponses())
	defer cancel()

	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		rsSpan := rs.At(i)

		// Extract Resource attributes, they will be added to every span.
		resourceAttrs := spanAttributesToMap(rsSpan.Resource().Attributes())

		ils := rsSpan.InstrumentationLibrarySpans()
		for j := 0; j < ils.Len(); j++ {
			ilsSpan := ils.At(j)
			spans := ilsSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				ev := e.builder.NewEvent()

				for k, v := range resourceAttrs {
					ev.AddField(k, v)
				}

				lib := ilsSpan.InstrumentationLibrary()
				if name := lib.Name(); name != "" {
					ev.AddField("library.name", name)
				}
				if version := lib.Version(); version != "" {
					ev.AddField("library.version", version)
				}

				if attrs := spanAttributesToMap(span.Attributes()); attrs != nil {
					for k, v := range attrs {
						ev.AddField(k, v)
					}

					e.addSampleRate(ev, attrs)
				}

				ev.Timestamp = timestampToTime(span.StartTimestamp())
				startTime := timestampToTime(span.StartTimestamp())
				endTime := timestampToTime(span.EndTimestamp())

				ev.Add(event{
					ID:            getHoneycombSpanID(span.SpanID()),
					TraceID:       getHoneycombTraceID(span.TraceID()),
					ParentID:      getHoneycombSpanID(span.ParentSpanID()),
					Name:          span.Name(),
					DurationMilli: float64(endTime.Sub(startTime)) / float64(time.Millisecond),
				})

				e.sendMessageEvents(span, resourceAttrs)
				e.sendSpanLinks(span)

				ev.AddField("span_kind", getSpanKind(span.Kind()))
				ev.AddField("status.code", getStatusCode(span.Status()))
				ev.AddField("status.message", getStatusMessage(span.Status()))

				if err := ev.SendPresampled(); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return consumererror.Combine(errs)
}

func getSpanKind(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindClient:
		return "client"
	case pdata.SpanKindServer:
		return "server"
	case pdata.SpanKindProducer:
		return "producer"
	case pdata.SpanKindConsumer:
		return "consumer"
	case pdata.SpanKindInternal:
		return "internal"
	case pdata.SpanKindUnspecified:
		fallthrough
	default:
		return "unspecified"
	}
}

// sendSpanLinks gets the list of links associated with this span and sends them as
// separate events to Honeycomb, with a span type "link".
func (e *honeycombExporter) sendSpanLinks(span pdata.Span) {
	links := span.Links()

	for i := 0; i < links.Len(); i++ {
		l := links.At(i)

		ev := e.builder.NewEvent()
		ev.Add(link{
			TraceID:        getHoneycombTraceID(span.TraceID()),
			ParentID:       getHoneycombSpanID(span.SpanID()),
			LinkTraceID:    getHoneycombTraceID(l.TraceID()),
			LinkSpanID:     getHoneycombSpanID(l.SpanID()),
			AnnotationType: "link",
		})
		attrs := spanAttributesToMap(l.Attributes())
		for k, v := range attrs {
			ev.AddField(k, v)
		}
		e.addSampleRate(ev, attrs)

		if err := ev.SendPresampled(); err != nil {
			e.onError(err)
		}
	}
}

// sendMessageEvents gets the list of timeevents from the span and sends them as
// separate events to Honeycomb, with a span type "span_event".
func (e *honeycombExporter) sendMessageEvents(span pdata.Span, resourceAttrs map[string]interface{}) {
	timeEvents := span.Events()

	for i := 0; i < timeEvents.Len(); i++ {
		event := timeEvents.At(i)

		ts := timestampToTime(event.Timestamp())
		name := event.Name()
		attrs := spanAttributesToMap(event.Attributes())

		// treat trace level fields as underlays with same keyed span attributes taking precedence.
		ev := e.builder.NewEvent()
		for k, v := range resourceAttrs {
			ev.AddField(k, v)
		}

		for k, v := range attrs {
			ev.AddField(k, v)
		}
		e.addSampleRate(ev, attrs)

		ev.Timestamp = ts
		ev.Add(spanEvent{
			Name:           name,
			TraceID:        getHoneycombTraceID(span.TraceID()),
			ParentID:       getHoneycombSpanID(span.SpanID()),
			ParentName:     span.Name(),
			AnnotationType: "span_event",
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

func (e *honeycombExporter) addSampleRate(event *libhoney.Event, attrs map[string]interface{}) {
	if e.sampleRateAttribute != "" && attrs != nil {
		if value, ok := attrs[e.sampleRateAttribute]; ok {
			switch v := value.(type) {
			case int64:
				event.SampleRate = uint(v)
			default:
				return
			}
		}
	}
}
