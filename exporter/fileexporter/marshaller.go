// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"bytes"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SmallJSONMarshaler struct{}

func jsonRepresentation(val pcommon.Value) string {
	switch val.Type() {
	case pcommon.ValueTypeStr:
		return fmt.Sprintf("%q", val.AsString())
	case pcommon.ValueTypeBool:
		return fmt.Sprint(val.Bool())
	case pcommon.ValueTypeInt:
		return fmt.Sprint(val.Int())
	case pcommon.ValueTypeDouble:
		return fmt.Sprint(val.Double())
	case pcommon.ValueTypeSlice:
		buf := bytes.Buffer{}
		buf.WriteString("[")
		for i := 0; i < val.Slice().Len(); i++ {
			buf.WriteString(jsonRepresentation(val.Slice().At(i)))
			buf.WriteString(",")
		}
		buf.WriteString("]")
		return buf.String()
	case pcommon.ValueTypeMap:
		buf := bytes.Buffer{}
		buf.WriteString("{")
		val.Map().Range(func(k string, v pcommon.Value) bool {
			buf.WriteString(`"`)
			buf.WriteString(k)
			buf.WriteString(`":`)
			buf.WriteString(jsonRepresentation(v))
			buf.WriteString(",")
			return true
		},
		)
		buf.WriteString("}")
		return buf.String()
	}
	return ""
}

func (m *SmallJSONMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	buf := bytes.Buffer{}
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		scopeLogs := ld.ResourceLogs().At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logRecords := scopeLogs.At(j).LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				log := logRecords.At(k)
				buf.WriteString(`{"ts":`)
				buf.WriteString(fmt.Sprint(log.Timestamp().AsTime().UnixNano() / 1e6))
				buf.WriteString(`,"attrs":{`)
				log.Attributes().Range(func(k string, v pcommon.Value) bool {
					buf.WriteString(`"`)
					buf.WriteString(k)
					buf.WriteString(`":`)
					buf.WriteString(jsonRepresentation(v))
					buf.WriteString(",")
					return true
				})
				buf.WriteString(`},"body":`)
				buf.WriteString(log.Body().Str())
				buf.WriteString("}\n")
			}

		}
	}
	return buf.Bytes(), nil
}
func (m *SmallJSONMarshaler) MarshalMetrics(mt pmetric.Metrics) ([]byte, error) {
	buf := bytes.Buffer{}
	for i := 0; i < mt.ResourceMetrics().Len(); i++ {
		scopeMetrics := mt.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metrics := scopeMetrics.At(j)
			for k := 0; k < metrics.Metrics().Len(); k++ {
				metric := metrics.Metrics().At(k)
				buf.WriteString(`{"name":`)
				buf.WriteString(fmt.Sprintf("%q", metric.Name()))
				buf.WriteString(`,"description":`)
				buf.WriteString(fmt.Sprintf("%q", metric.Description()))
				buf.WriteString(`,"unit":`)
				buf.WriteString(fmt.Sprintf("%q", metric.Unit()))
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					buf.WriteString(`,"type":"gauge","value":`)
					buf.WriteString(fmt.Sprint(metric.Gauge().DataPoints().At(0).DoubleValue()))
				case pmetric.MetricTypeSum:
					buf.WriteString(`,"type":"sum","value":`)
					buf.WriteString(fmt.Sprint(metric.Sum().DataPoints().At(0).IntValue()))
				case pmetric.MetricTypeHistogram:
					buf.WriteString(`,"type":"histogram","value":`)
					// How to export a histogram metric??
				case pmetric.MetricTypeSummary:
					buf.WriteString(`,"type":"summary","value":`)
					// How to export a summary metric??
				case pmetric.MetricTypeExponentialHistogram:
					buf.WriteString(`,"type":"histogram","value":`)
					// How to export a exponential histogram metric??
				case pmetric.MetricTypeEmpty:
					// Nothing to do?
				}
				buf.WriteString("}\n")
			}
		}
	}
	return buf.Bytes(), nil
}
func (m *SmallJSONMarshaler) MarshalTraces(tr ptrace.Traces) ([]byte, error) {
	buf := bytes.Buffer{}
	for i := 0; i < tr.ResourceSpans().Len(); i++ {
		resourceSpan := tr.ResourceSpans().At(i)
		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)
			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)
				buf.WriteString(`{"trace_id":`)
				buf.WriteString(fmt.Sprintf("%q", span.TraceID().String()))
				buf.WriteString(`,"span_id":`)
				buf.WriteString(fmt.Sprintf("%q", span.SpanID().String()))
				buf.WriteString(`,"parent_span_id":`)
				buf.WriteString(fmt.Sprintf("%q", span.ParentSpanID().String()))
				buf.WriteString(`,"name":`)
				buf.WriteString(fmt.Sprintf("%q", span.Name()))
				buf.WriteString(`,"start_time":`)
				buf.WriteString(fmt.Sprint(float64(span.StartTimestamp().AsTime().UnixNano()) / 1e6))
				buf.WriteString(`,"end_time":`)
				buf.WriteString(fmt.Sprint(float64(span.EndTimestamp().AsTime().UnixNano()) / 1e6))
				buf.WriteString(`,"attrs":{`)
				span.Attributes().Range(func(k string, v pcommon.Value) bool {
					buf.WriteString(`"`)
					buf.WriteString(k)
					buf.WriteString(`":`)
					buf.WriteString(jsonRepresentation(v))
					buf.WriteString(",")
					return true
				})
				buf.WriteString(`},"events":[`)
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					buf.WriteString(`{"time":`)
					buf.WriteString(fmt.Sprint(float64(event.Timestamp().AsTime().UnixNano()) / 1e6))
					buf.WriteString(`,"attrs":{`)
					event.Attributes().Range(func(k string, v pcommon.Value) bool {
						buf.WriteString(`"`)
						buf.WriteString(k)
						buf.WriteString(`":`)
						buf.WriteString(jsonRepresentation(v))
						buf.WriteString(",")
						return true
					})
					buf.WriteString("}},")
				}
				buf.WriteString(`],"links":[`)
				for l := 0; l < span.Links().Len(); l++ {
					link := span.Links().At(l)
					buf.WriteString(`{"trace_id":`)
					buf.WriteString(fmt.Sprintf("%q", link.TraceID().String()))
					buf.WriteString(`,"span_id":`)
					buf.WriteString(fmt.Sprintf("%q", link.SpanID().String()))
					buf.WriteString(`,"attrs":{`)
					link.Attributes().Range(func(k string, v pcommon.Value) bool {
						buf.WriteString(`"`)
						buf.WriteString(k)
						buf.WriteString(`":`)
						buf.WriteString(jsonRepresentation(v))
						buf.WriteString(",")
						return true
					})
					buf.WriteString("}},")
				}
				buf.WriteString("]}\n")
			}
		}
	}
	return buf.Bytes(), nil

}

// Marshaler configuration used for marhsaling Protobuf
var tracesMarshalers = map[string]ptrace.Marshaler{
	formatTypeJSON:      &ptrace.JSONMarshaler{},
	formatTypeProto:     &ptrace.ProtoMarshaler{},
	formatTypeSmallJSON: &SmallJSONMarshaler{},
}
var metricsMarshalers = map[string]pmetric.Marshaler{
	formatTypeJSON:      &pmetric.JSONMarshaler{},
	formatTypeProto:     &pmetric.ProtoMarshaler{},
	formatTypeSmallJSON: &SmallJSONMarshaler{},
}
var logsMarshalers = map[string]plog.Marshaler{
	formatTypeJSON:      &plog.JSONMarshaler{},
	formatTypeProto:     &plog.ProtoMarshaler{},
	formatTypeSmallJSON: &SmallJSONMarshaler{},
}

type marshaller struct {
	tracesMarshaler  ptrace.Marshaler
	metricsMarshaler pmetric.Marshaler
	logsMarshaler    plog.Marshaler

	compression string
	compressor  compressFunc

	formatType string
}

func newMarshaller(conf *Config, host component.Host) (*marshaller, error) {
	if conf.Encoding != nil {
		encoding := host.GetExtensions()[*conf.Encoding]
		if encoding == nil {
			return nil, fmt.Errorf("unknown encoding %q", conf.Encoding)
		}
		// cast with ok to avoid panics.
		tm, _ := encoding.(ptrace.Marshaler)
		pm, _ := encoding.(pmetric.Marshaler)
		lm, _ := encoding.(plog.Marshaler)
		return &marshaller{
			tracesMarshaler:  tm,
			metricsMarshaler: pm,
			logsMarshaler:    lm,
			compression:      conf.Compression,
			compressor:       buildCompressor(conf.Compression),
		}, nil
	}
	return &marshaller{
		formatType:       conf.FormatType,
		tracesMarshaler:  tracesMarshalers[conf.FormatType],
		metricsMarshaler: metricsMarshalers[conf.FormatType],
		logsMarshaler:    logsMarshalers[conf.FormatType],
		compression:      conf.Compression,
		compressor:       buildCompressor(conf.Compression),
	}, nil
}

func (m *marshaller) marshalTraces(td ptrace.Traces) ([]byte, error) {
	if m.tracesMarshaler == nil {
		return nil, errors.New("traces are not supported by encoding")
	}
	buf, err := m.tracesMarshaler.MarshalTraces(td)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}

func (m *marshaller) marshalMetrics(md pmetric.Metrics) ([]byte, error) {
	if m.metricsMarshaler == nil {
		return nil, errors.New("metrics are not supported by encoding")
	}
	buf, err := m.metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}

func (m *marshaller) marshalLogs(ld plog.Logs) ([]byte, error) {
	if m.logsMarshaler == nil {
		return nil, errors.New("logs are not supported by encoding")
	}
	buf, err := m.logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	buf = m.compressor(buf)
	return buf, nil
}
