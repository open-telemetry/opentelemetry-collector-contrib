package logtospanexporter

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
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
		config.FieldMap.ServiceName,
		config.FieldMap.SpanStartTime,
		config.FieldMap.SpanEndTime,
		config.FieldMap.W3CTraceContextFields.Traceparent,
		config.FieldMap.W3CTraceContextFields.Tracestate)

	return func(ctx context.Context, ld pdata.Logs) error {
		outTraces := pdata.NewTraces()

		defaultSpanSlice := pdata.NewSpanSlice()
		// Span groups based on the ServiceName found in log message.
		spanGroups := map[string]*pdata.SpanSlice{tracetranslator.ResourceNoServiceName: &defaultSpanSlice}

		var errs []error

		rls := ld.ResourceLogs()
		for i := 0; i < rls.Len(); i++ {
			ills := rls.At(i).InstrumentationLibraryLogs()
			for j := 0; j < ills.Len(); j++ {
				logs := ills.At(j).Logs()
				for k := 0; k < logs.Len(); k++ {
					log := logs.At(k)

					serviceName, span, err := convertLogToSpan(&log, config)
					if err != nil {
						continue
					}

					if spanGroup, ok := spanGroups[serviceName]; !ok {
						spanGroup.Append(*span)
						continue
					}

					sps := pdata.NewSpanSlice()
					sps.Append(*span)
					spanGroups[serviceName] = &sps
				}
			}
		}

		outTraces.ResourceSpans().Resize(len(spanGroups))

		sgi := 0
		for serviceName, v := range spanGroups {
			outRs := outTraces.ResourceSpans().At(sgi)
			outRs.Resource().Attributes().InsertString(conventions.AttributeServiceName, serviceName)
			outRs.InstrumentationLibrarySpans().Resize(1)
			outIls := outRs.InstrumentationLibrarySpans().At(0)

			outIls.InstrumentationLibrary().SetName(ilName)
			outIls.InstrumentationLibrary().SetVersion(params.ApplicationStartInfo.Version)

			v.CopyTo(outIls.Spans())

			sgi++
		}

		if err := tracesExp.ConsumeTraces(ctx, outTraces); err != nil {
			errs = append(errs, err)
		}

		return consumererror.Combine(errs)
	}
}

func convertLogToSpan(log *pdata.LogRecord, config *Config) (string, *pdata.Span, error) {
	spc, err := extractSpanContext(log, config)
	if err != nil {
		return "", nil, err
	}

	if !spc.IsSampled() {
		// TODO: add custom metric?
		return "", nil, fmt.Errorf("trace not sampled")
	}

	span := pdata.NewSpan()
	span.SetTraceID(pdata.NewTraceID(spc.TraceID()))
	span.SetSpanID(pdata.NewSpanID(spc.SpanID()))

	if spanKind, ok := log.Attributes().Get(config.FieldMap.SpanKind); ok {
		switch spanKind.StringVal() {
		case "server":
			span.SetKind(pdata.SpanKindSERVER)
		case "client":
			span.SetKind(pdata.SpanKindCLIENT)
		case "consumer":
			span.SetKind(pdata.SpanKindCONSUMER)
		case "producer":
			span.SetKind(pdata.SpanKindPRODUCER)
		case "internal":
			span.SetKind(pdata.SpanKindINTERNAL)
		default:
			span.SetKind(pdata.SpanKindSERVER)
		}
	} else {
		span.SetKind(pdata.SpanKindSERVER)
	}

	spanName, ok := log.Attributes().Get(config.FieldMap.SpanName)
	if !ok {
		return "", nil, fmt.Errorf("%s was not found in log attributes", config.FieldMap.SpanName)
	}
	span.SetName(spanName.StringVal())

	sts, err := timestampFromField(log, config.FieldMap.SpanStartTime, config.TimeFormat)
	if err != nil {
		// TODO: add custom metric?
		return "", nil, err
	}
	span.SetStartTimestamp(sts)

	ets, err := timestampFromField(log, config.FieldMap.SpanEndTime, config.TimeFormat)
	if err != nil {
		// TODO: add custom metric?
		return "", nil, err
	}
	span.SetEndTimestamp(ets)

	log.Attributes().CopyTo(span.Attributes())
	for k := range config.FieldMap.Ignored {
		_ = span.Attributes().Delete(config.FieldMap.Ignored[k])
	}

	serviceName, ok := log.Attributes().Get(config.FieldMap.ServiceName)
	if !ok {
		return tracetranslator.ResourceNoServiceName, &span, nil
	}

	return serviceName.StringVal(), &span, nil
}

func timestampFromField(log *pdata.LogRecord, fieldName string, format TimeFormat) (pdata.Timestamp, error) {
	if startTime, ok := log.Attributes().Get(fieldName); ok {
		ts, err := convertTimestamp(startTime.StringVal(), format)
		if err != nil {
			return 0, err
		}

		return ts, nil
	}

	return 0, fmt.Errorf("%s not found in log attributes", fieldName)
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
	w3cTraceparentHeader = "traceparent"
	w3cTracestateHeader  = "tracestate"
)

func extractSpanContext(log *pdata.LogRecord, config *Config) (*trace.SpanContext, error) {
	switch config.TraceType {
	case W3CTraceType:
		// TODO: Determine better way to reuse existing libs for trace context.
		hc := propagation.HeaderCarrier{}
		if traceparent, ok := log.Attributes().Get(config.FieldMap.W3CTraceContextFields.Traceparent); ok {
			hc.Set(w3cTraceparentHeader, traceparent.StringVal())
		}

		if tracestate, ok := log.Attributes().Get(config.FieldMap.W3CTraceContextFields.Tracestate); ok {
			hc.Set(w3cTracestateHeader, tracestate.StringVal())
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
