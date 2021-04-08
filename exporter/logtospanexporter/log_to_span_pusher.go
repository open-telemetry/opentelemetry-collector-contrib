package logtospanexporter

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TODO: Implement custom metrics.
func createLogToSpanPusher(config *Config, params component.ExporterCreateParams, tracesExp component.TracesExporter) exporterhelper.PushLogs {
	// TODO: Determine better name format
	ilName := fmt.Sprintf("%s ('%s' exporter)", params.ApplicationStartInfo.LongName, typeStr)

	config.FieldMap.Ignored = append(
		config.FieldMap.Ignored,
		// These fields are used to create a span and don't provide additional value as span attributes.
		config.FieldMap.SpanStartTime,
		config.FieldMap.SpanEndTime,
		config.FieldMap.SpanName,
		config.FieldMap.W3CTraceContextFields.Traceparent,
		config.FieldMap.W3CTraceContextFields.Tracestate)

	return func(ctx context.Context, ld pdata.Logs) error {
		outTraces := pdata.NewTraces()
		outTraces.ResourceSpans().Resize(1)
		outRs := outTraces.ResourceSpans().At(0)
		outRs.InstrumentationLibrarySpans().Resize(1)
		outIls := outRs.InstrumentationLibrarySpans().At(0)

		outIls.InstrumentationLibrary().SetName(ilName)
		outIls.InstrumentationLibrary().SetVersion(params.ApplicationStartInfo.Version)

		var errs []error

		rls := ld.ResourceLogs()
		for i := 0; i < rls.Len(); i++ {
			ills := rls.At(i).InstrumentationLibraryLogs()
			for j := 0; j < ills.Len(); j++ {
				logs := ills.At(j).Logs()
				for k := 0; k < logs.Len(); k++ {
					log := logs.At(k)

					spc, err := extractSpanContext(log, config)
					if err != nil {
						// TODO: add custom metric and log?
						continue
					}

					if !spc.IsSampled() {
						// TODO: add custom metric?
						continue
					}

					span := pdata.NewSpan()
					span.SetTraceID(pdata.NewTraceID(spc.TraceID()))
					span.SetSpanID(pdata.NewSpanID(spc.SpanID()))
					span.SetKind(pdata.SpanKindSERVER)

					if name, ok := log.Attributes().Get(config.FieldMap.SpanName); ok {
						span.SetName(name.StringVal())
					} else {
						// TODO: Should we exit? Default name?
					}

					if startTime, ok := log.Attributes().Get(config.FieldMap.SpanStartTime); ok {
						if ts, err := convertTimestamp(startTime.StringVal(), config.TimeFormat); err == nil {
							span.SetStartTime(ts)
						}
					} else {
						// TODO: add custom metric and log?
						continue
					}

					if endTime, ok := log.Attributes().Get(config.FieldMap.SpanEndTime); ok {
						if ts, err := convertTimestamp(endTime.StringVal(), config.TimeFormat); err == nil {
							span.SetEndTime(ts)
						}
					} else {
						// TODO: add custom metric and log?
						continue
					}

					log.Attributes().CopyTo(span.Attributes())
					for k := range config.FieldMap.Ignored {
						_ = span.Attributes().Delete(config.FieldMap.Ignored[k])
					}

					outIls.Spans().Append(span)
				}
			}
		}

		if err := tracesExp.ConsumeTraces(ctx, outTraces); err != nil {
			errs = append(errs, err)
		}

		return consumererror.CombineErrors(errs)
	}
}

func convertTimestamp(timestamp string, format TimeFormat) (pdata.Timestamp, error) {
	switch format {
	case UnixEpochNanoTimeFormat:
		if nanoTime, err := strconv.ParseUint(timestamp, 10, 64); err == nil {
			return pdata.Timestamp(nanoTime), nil
		}
	case UnixEpochMicroTimeFormat:
		if microTime, err := strconv.ParseUint(timestamp, 10, 64); err == nil {
			return pdata.Timestamp(microTime * uint64(1000000)), nil
		}
	}

	return 0, fmt.Errorf("invalid timestammp")
}

const (
	traceparentHeader = "traceparent"
	tracestateHeader  = "tracestate"
)

func extractSpanContext(log pdata.LogRecord, config *Config) (*trace.SpanContext, error) {
	switch config.TraceType {
	case W3CTraceType:
		// TODO: Determine better way to reuse existing libs for trace context.
		hc := propagation.HeaderCarrier{}
		if traceparent, ok := log.Attributes().Get(config.FieldMap.W3CTraceContextFields.Traceparent); ok {
			hc.Set(traceparentHeader, traceparent.StringVal())
		}

		if tracestate, ok := log.Attributes().Get(config.FieldMap.W3CTraceContextFields.Tracestate); ok {
			hc.Set(tracestateHeader, tracestate.StringVal())
		}

		tc := propagation.TraceContext{}
		c := tc.Extract(context.Background(), hc)
		spc := trace.RemoteSpanContextFromContext(c)

		if spc.IsValid() {
			return &spc, nil
		}
	}

	return nil, fmt.Errorf("log did not contain a valid span context")
}
