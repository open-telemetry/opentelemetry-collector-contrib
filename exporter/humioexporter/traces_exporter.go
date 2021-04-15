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
	"sync"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

// HumioLink represents a relation between two spans
type HumioLink struct {
	TraceId    string `json:"trace_id"`
	SpanId     string `json:"span_id"`
	TraceState string `json:"state"`
}

// HumioSpan represents a span as it is stored inside Humio
type HumioSpan struct {
	TraceId           string                 `json:"trace_id"`
	SpanId            string                 `json:"span_id"`
	ParentSpanId      string                 `json:"parent_id,omitempty"`
	Name              string                 `json:"name"`
	Kind              string                 `json:"kind"`
	Start             int64                  `json:"start"`
	End               int64                  `json:"end"`
	StatusCode        string                 `json:"status"`
	StatusDescription string                 `json:"status_descr"`
	ServiceName       string                 `json:"service"`
	Links             []*HumioLink           `json:"links"`
	Attributes        map[string]interface{} `json:"attributes"`
}

type humioTracesExporter struct {
	cfg    *Config
	logger *zap.Logger
	client exporterClient
	wg     sync.WaitGroup
}

func newTracesExporter(cfg *Config, logger *zap.Logger, client exporterClient) *humioTracesExporter {
	return &humioTracesExporter{
		cfg:    cfg,
		logger: logger,
		client: client,
	}
}

func (e *humioTracesExporter) pushTraceData(ctx context.Context, td pdata.Traces) error {
	e.wg.Add(1)
	defer e.wg.Done()

	evts, err := e.tracesToHumioEvents(td)
	if err != nil {
		return consumererror.Permanent(err)
	}

	err = e.client.sendStructuredEvents(ctx, evts)
	if err == nil {
		return nil
	}

	if consumererror.IsPermanent(err) {
		return err
	}

	return consumererror.NewTraces(err, td)
}

func (e *humioTracesExporter) tracesToHumioEvents(td pdata.Traces) ([]*HumioStructuredEvents, error) {
	results := make([]*HumioStructuredEvents, 0, td.ResourceSpans().Len())

	// Each resource describes unique origin that generates spans
	resSpans := td.ResourceSpans()
	for i := 0; i < resSpans.Len(); i++ {
		resSpan := resSpans.At(i)
		r := resSpan.Resource()

		serviceName := ""
		if sName, ok := r.Attributes().Get(conventions.AttributeServiceName); ok {
			serviceName = sName.StringVal()
		}

		evts := make([]*HumioStructuredEvent, 0, resSpan.InstrumentationLibrarySpans().Len())

		// For each resource, spans are grouped by the instrumentation library (plugin) that generated them
		instSpans := resSpan.InstrumentationLibrarySpans()
		for j := 0; j < instSpans.Len(); j++ {
			instSpan := instSpans.At(j)
			lib := instSpan.InstrumentationLibrary()

			// Lastly, we get access to the actual spans
			otelSpans := instSpan.Spans()
			for k := 0; k < otelSpans.Len(); k++ {
				otelSpan := otelSpans.At(k)

				evts = append(evts, e.spanToHumioEvent(otelSpan, lib, r))
			}
		}

		// TODO: More user control and adherence to conventions for tags
		results = append(results, &HumioStructuredEvents{
			Tags: map[string]string{
				"service": serviceName,
			},
			Events: evts,
		})
	}

	return results, nil
}

func (e *humioTracesExporter) spanToHumioEvent(span pdata.Span, inst pdata.InstrumentationLibrary, res pdata.Resource) *HumioStructuredEvent {
	attr := toHumioAttributes(span.Attributes(), res.Attributes())
	attr["instrumentationLibrary.name"] = inst.Name()
	attr["instrumentationLibrary.version"] = inst.Version()

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
			TraceId:           span.TraceID().HexString(),
			SpanId:            span.SpanID().HexString(),
			ParentSpanId:      span.ParentSpanID().HexString(),
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
			TraceId:    link.TraceID().HexString(),
			SpanId:     link.SpanID().HexString(),
			TraceState: string(link.TraceState()),
		})
	}
	return links
}

func toHumioAttributes(attrMaps ...pdata.AttributeMap) map[string]interface{} {
	attr := make(map[string]interface{}, 0)
	for _, attrMap := range attrMaps {
		attrMap.ForEach(func(k string, v pdata.AttributeValue) {
			attr[k] = toHumioAttributeValue(v)
		})
	}
	return attr
}

func toHumioAttributeValue(rawVal pdata.AttributeValue) interface{} {
	switch rawVal.Type() {
	case pdata.AttributeValueSTRING:
		return rawVal.StringVal()
	case pdata.AttributeValueINT:
		return rawVal.IntVal()
	case pdata.AttributeValueDOUBLE:
		return rawVal.DoubleVal()
	case pdata.AttributeValueBOOL:
		return rawVal.BoolVal()
	case pdata.AttributeValueMAP:
		return toHumioAttributes(rawVal.MapVal())
	case pdata.AttributeValueARRAY:
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

func (e *humioTracesExporter) shutdown(context.Context) error {
	e.wg.Wait()
	return nil
}
