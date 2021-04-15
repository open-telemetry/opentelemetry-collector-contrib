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

type HumioLink struct {
	TraceId    string `json:"trace_id"`
	SpanId     string `json:"span_id"`
	TraceState string `json:"state"`
}

type HumioSpan struct {
	TraceId           string            `json:"trace_id"`
	SpanId            string            `json:"span_id"`
	ParentSpanId      string            `json:"parent_id,omitempty"`
	Name              string            `json:"name"`
	Kind              string            `json:"kind"`
	Start             int64             `json:"start"`
	End               int64             `json:"end"`
	StatusCode        string            `json:"status"`
	StatusDescription string            `json:"status_descr"`
	ServiceName       string            `json:"service"`
	Links             HumioLink         `json:"links"`
	Attributes        map[string]string `json:"attributes"`
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
		// TODO: All additional attributes should propably be dumped as well

		evts := make([]*HumioStructuredEvent, 0, resSpan.InstrumentationLibrarySpans().Len())

		// For each resource, spans are grouped by the instrumentation library (plugin) that generated them
		instSpans := resSpan.InstrumentationLibrarySpans()
		for j := 0; j < instSpans.Len(); j++ {
			instSpan := instSpans.At(j)
			// TODO: Use
			// lib := instSpan.InstrumentationLibrary()
			// lib.Name()
			// lib.Version()

			// Lastly, we get access to the actual spans
			otelSpans := instSpan.Spans()
			for k := 0; k < otelSpans.Len(); k++ {
				otelSpan := otelSpans.At(k)

				evts = append(evts, &HumioStructuredEvent{
					Timestamp: otelSpan.StartTimestamp().AsTime(),
					AsUnix:    e.cfg.Traces.UnixTimestamps,
					Attributes: &HumioSpan{
						TraceId:           otelSpan.TraceID().HexString(),
						SpanId:            otelSpan.SpanID().HexString(),
						ParentSpanId:      otelSpan.ParentSpanID().HexString(),
						Name:              otelSpan.Name(),
						Kind:              otelSpan.Kind().String(),
						Start:             otelSpan.StartTimestamp().AsTime().UnixNano(),
						End:               otelSpan.EndTimestamp().AsTime().UnixNano(),
						StatusCode:        otelSpan.Status().Code().String(),
						StatusDescription: otelSpan.Status().Message(),
						ServiceName:       serviceName,
						// TODO: Links
						// TODO: Dump additional attributes
					},
				})
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

func (e *humioTracesExporter) shutdown(context.Context) error {
	e.wg.Wait()
	return nil
}
