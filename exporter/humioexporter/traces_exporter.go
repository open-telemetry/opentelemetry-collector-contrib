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

package humioexporter

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
)

// HumioLink represents a relation between two spans
type HumioLink struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceState string `json:"state,omitempty"`
}

// HumioSpan represents a span as it is stored inside Humio
type HumioSpan struct {
	TraceID           string                 `json:"trace_id"`
	SpanID            string                 `json:"span_id"`
	ParentSpanID      string                 `json:"parent_id,omitempty"`
	Name              string                 `json:"name"`
	Kind              string                 `json:"kind"`
	Start             int64                  `json:"start"`
	End               int64                  `json:"end"`
	StatusCode        string                 `json:"status,omitempty"`
	StatusDescription string                 `json:"status_descr,omitempty"`
	ServiceName       string                 `json:"service"`
	Links             []*HumioLink           `json:"links,omitempty"`
	Attributes        map[string]interface{} `json:"attributes,omitempty"`
}

type humioTracesExporter struct {
	cfg    *Config
	logger *zap.Logger
	client exporterClient
	wg     sync.WaitGroup

	// Needed to enable current unit tests with the latest changes from core collector.
	getClient clientGetter
}

type clientGetter func(cfg *Config, logger *zap.Logger, host component.Host) (exporterClient, error)

func newTracesExporter(cfg *Config, logger *zap.Logger) *humioTracesExporter {
	gc := func(cfg *Config, logger *zap.Logger, host component.Host) (exporterClient, error) {
		client, err := newHumioClient(cfg, logger, host)
		if err != nil {
			return nil, err
		}

		return client, nil
	}

	return &humioTracesExporter{
		cfg:       cfg,
		logger:    logger,
		getClient: gc,
	}
}

func newTracesExporterWithClientGetter(cfg *Config, logger *zap.Logger, cg clientGetter) *humioTracesExporter {
	return &humioTracesExporter{
		cfg:       cfg,
		logger:    logger,
		getClient: cg,
	}
}

func (e *humioTracesExporter) pushTraceData(ctx context.Context, td pdata.Traces) error {
	e.wg.Add(1)
	defer e.wg.Done()

	evts, conversionErr := e.tracesToHumioEvents(td)
	if conversionErr != nil && len(evts) == 0 {
		// All traces failed conversion - no need to retry any more since this is not a
		// transient failure. By raising a permanent error, the queued retry middleware
		// will expose a metric for failed spans immediately
		return consumererror.NewPermanent(conversionErr)
	}

	err := e.client.sendStructuredEvents(ctx, evts)
	if err != nil {
		// Just forward the error from the client if the request failed
		return err
	}

	// Successfully sent some traces, report any subset that failed conversion (if any).
	// While we know that retrying will be unsuccessful, raising a permanent error at
	// this time will cause the queued retry middleware to assume that all the spans
	// failed conversion (which is not the case), resulting an incorrect metric being
	// reported. By instead raising a transient error, the middleware will construct a
	// new request of only the problematic spans, which will fail entirely. When this
	// case is handled above, a permanent error is reported to the middleware, and the
	// correct metric of failed spans is generated.
	return conversionErr
}

func (e *humioTracesExporter) tracesToHumioEvents(td pdata.Traces) ([]*HumioStructuredEvents, error) {
	organizer := newTagOrganizer(e.cfg.Tag, tagFromSpan)
	var droppedTraces []pdata.ResourceSpans

	resSpans := td.ResourceSpans()
	for i := 0; i < resSpans.Len(); i++ {
		resSpan := resSpans.At(i)
		r := resSpan.Resource()

		if _, ok := r.Attributes().Get(conventions.AttributeServiceName); !ok {
			droppedTraces = append(droppedTraces, resSpan)
			e.logger.Error("skipping export of spans for resource with missing service name, which is required for the Humio exporter")
			continue
		}

		instSpans := resSpan.InstrumentationLibrarySpans()
		for j := 0; j < instSpans.Len(); j++ {
			instSpan := instSpans.At(j)
			lib := instSpan.InstrumentationLibrary()

			otelSpans := instSpan.Spans()
			for k := 0; k < otelSpans.Len(); k++ {
				otelSpan := otelSpans.At(k)
				organizer.consume(e.spanToHumioEvent(otelSpan, lib, r))
			}
		}
	}

	results := organizer.asEvents()

	if len(droppedTraces) > 0 {
		dropped := pdata.NewTraces()
		for _, t := range droppedTraces {
			tgt := dropped.ResourceSpans().AppendEmpty()
			t.CopyTo(tgt)
		}

		return results, consumererror.NewTraces(
			errors.New("unable to serialize spans due to missing required service name for the associated resource"),
			dropped,
		)
	}

	return results, nil
}

func (e *humioTracesExporter) spanToHumioEvent(span pdata.Span, inst pdata.InstrumentationLibrary, res pdata.Resource) *HumioStructuredEvent {
	attr := toHumioAttributes(span.Attributes(), res.Attributes())
	if instName := inst.Name(); instName != "" {
		attr[conventions.InstrumentationLibraryName] = instName
	}
	if instVer := inst.Version(); instVer != "" {
		attr[conventions.InstrumentationLibraryVersion] = instVer
	}

	serviceName := ""
	if sName, ok := res.Attributes().Get(conventions.AttributeServiceName); ok {
		// No need to store the service name in two places
		delete(attr, conventions.AttributeServiceName)
		serviceName = sName.StringVal()
	}

	return &HumioStructuredEvent{
		Timestamp: span.StartTimestamp().AsTime(),
		AsUnix:    e.cfg.Traces.UnixTimestamps,
		Attributes: &HumioSpan{
			TraceID:           span.TraceID().HexString(),
			SpanID:            span.SpanID().HexString(),
			ParentSpanID:      span.ParentSpanID().HexString(),
			Name:              span.Name(),
			Kind:              span.Kind().String(),
			Start:             span.StartTimestamp().AsTime().UnixNano(),
			End:               span.EndTimestamp().AsTime().UnixNano(),
			StatusCode:        span.Status().Code().String(),
			StatusDescription: span.Status().Message(),
			ServiceName:       serviceName,
			Links:             toHumioLinks(span.Links()),
			Attributes:        attr,
		},
	}
}

func toHumioLinks(pLinks pdata.SpanLinkSlice) []*HumioLink {
	links := make([]*HumioLink, 0, pLinks.Len())
	for i := 0; i < pLinks.Len(); i++ {
		link := pLinks.At(i)
		links = append(links, &HumioLink{
			TraceID:    link.TraceID().HexString(),
			SpanID:     link.SpanID().HexString(),
			TraceState: string(link.TraceState()),
		})
	}
	return links
}

func toHumioAttributes(attrMaps ...pdata.AttributeMap) map[string]interface{} {
	attr := make(map[string]interface{})
	for _, attrMap := range attrMaps {
		attrMap.Range(func(k string, v pdata.AttributeValue) bool {
			attr[k] = toHumioAttributeValue(v)
			return true
		})
	}
	return attr
}

func toHumioAttributeValue(rawVal pdata.AttributeValue) interface{} {
	switch rawVal.Type() {
	case pdata.AttributeValueTypeString:
		return rawVal.StringVal()
	case pdata.AttributeValueTypeInt:
		return rawVal.IntVal()
	case pdata.AttributeValueTypeDouble:
		return rawVal.DoubleVal()
	case pdata.AttributeValueTypeBool:
		return rawVal.BoolVal()
	case pdata.AttributeValueTypeMap:
		return toHumioAttributes(rawVal.MapVal())
	case pdata.AttributeValueTypeArray:
		arrVal := rawVal.ArrayVal()
		arr := make([]interface{}, 0, arrVal.Len())
		for i := 0; i < arrVal.Len(); i++ {
			arr = append(arr, toHumioAttributeValue(arrVal.At(i)))
		}
		return arr
	}

	// Also handles AttributeValueNULL
	return nil
}

func tagFromSpan(evt *HumioStructuredEvent, strategy Tagger) string {
	switch strategy {
	case TagTraceID:
		return evt.Attributes.(*HumioSpan).TraceID

	case TagServiceName:
		return evt.Attributes.(*HumioSpan).ServiceName

	default: // TagNone
		return ""
	}
}

// start starts the exporter
func (e *humioTracesExporter) start(_ context.Context, host component.Host) error {
	client, err := e.getClient(e.cfg, e.logger, host)
	if err != nil {
		return err
	}

	e.client = client

	return nil
}

func (e *humioTracesExporter) shutdown(context.Context) error {
	e.wg.Wait()
	return nil
}
