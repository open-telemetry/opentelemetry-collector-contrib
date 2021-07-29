// Copyright OpenTelemetry Authors
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

package uptraceexporter

import (
	"context"

	"github.com/uptrace/uptrace-go/spanexp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type traceExporter struct {
	cfg    *Config
	logger *zap.Logger
	upexp  *spanexp.Exporter
}

func newTracesExporter(cfg *Config, logger *zap.Logger) *traceExporter {
	return &traceExporter{
		cfg:    cfg,
		logger: logger,
	}
}

// pushTraceData is the method called when trace data is available.
func (e *traceExporter) pushTraceData(ctx context.Context, traces pdata.Traces) error {
	outSpans := make([]spanexp.Span, 0, traces.SpanCount())

	rsSpans := traces.ResourceSpans()
	for i := 0; i < rsSpans.Len(); i++ {
		rsSpan := rsSpans.At(i)
		resource := e.keyValueSlice(rsSpan.Resource().Attributes())

		ils := rsSpan.InstrumentationLibrarySpans()
		for j := 0; j < ils.Len(); j++ {
			ilsSpan := ils.At(j)
			lib := ilsSpan.InstrumentationLibrary()

			spans := ilsSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				outSpans = append(outSpans, spanexp.Span{})
				out := &outSpans[len(outSpans)-1]

				out.ID = asUint64(span.SpanID().Bytes())
				out.ParentID = asUint64(span.ParentSpanID().Bytes())
				out.TraceID = span.TraceID().Bytes()

				out.Name = span.Name()
				out.Kind = spanKind(span.Kind())
				out.StartTime = int64(span.StartTimestamp())
				out.EndTime = int64(span.EndTimestamp())

				out.Resource = resource
				out.Attrs = e.keyValueSlice(span.Attributes())

				out.StatusCode = statusCode(span.Status().Code())
				out.StatusMessage = span.Status().Message()

				out.TracerName = lib.Name()
				out.TracerVersion = lib.Version()

				out.Events = e.uptraceEvents(span.Events())
				out.Links = e.uptraceLinks(span.Links())
			}
		}
	}

	if len(outSpans) == 0 {
		return nil
	}

	out := map[string]interface{}{
		"spans": outSpans,
	}
	if err := e.upexp.SendSpans(ctx, out); err != nil {
		if err, ok := err.(temporaryError); ok && err.Temporary() {
			return err
		}
		return consumererror.Permanent(err)
	}

	return nil
}

func (e *traceExporter) start(_ context.Context, host component.Host) error {
	if e.cfg.HTTPClientSettings.Endpoint != "" {
		e.logger.Warn("uptraceexporter: endpoint is not supported; use dsn instead")
	}

	client, err := e.cfg.HTTPClientSettings.ToClient(host.GetExtensions())
	if err != nil {
		return err
	}

	upexp, err := spanexp.NewExporter(&spanexp.Config{
		DSN:        e.cfg.DSN,
		HTTPClient: client,
		MaxRetries: -1, // disable retries because Collector already handles it
	})
	if err != nil {
		return err
	}

	e.upexp = upexp

	return nil
}

func (e *traceExporter) shutdown(ctx context.Context) error {
	return e.upexp.Shutdown(ctx)
}

type temporaryError interface {
	error
	Temporary() bool // Is the error temporary?
}
