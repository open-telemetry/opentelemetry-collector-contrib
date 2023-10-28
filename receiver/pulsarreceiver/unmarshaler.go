// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

var errUnrecognizedEncoding = errors.New("unrecognized encoding")

// copy from kafka receiver

// TracesUnmarshaler deserializes the message body.
type TracesUnmarshaler interface {
	// Unmarshal deserializes the message body into tracesConsumer.
	Unmarshal([]byte) (ptrace.Traces, error)

	// Encoding of the serialized messages.
	Encoding() string
}

// MetricsUnmarshaler deserializes the message body
type MetricsUnmarshaler interface {
	// Unmarshal deserializes the message body into tracesConsumer
	Unmarshal([]byte) (pmetric.Metrics, error)

	// Encoding of the serialized messages
	Encoding() string
}

// LogsUnmarshaler deserializes the message body.
type LogsUnmarshaler interface {
	// Unmarshal deserializes the message body into tracesConsumer.
	Unmarshal([]byte) (plog.Logs, error)

	// Encoding of the serialized messages.
	Encoding() string
}

// defaultTracesUnmarshalers returns map of supported encodings with TracesUnmarshaler.
func defaultTracesUnmarshalers() map[string]TracesUnmarshaler {
	otlpPb := newPdataTracesUnmarshaler(&ptrace.ProtoUnmarshaler{}, defaultEncoding)
	otlpJSON := newPdataTracesUnmarshaler(&ptrace.JSONUnmarshaler{}, "otlp_json")
	jaegerProto := jaegerProtoSpanUnmarshaler{}
	jaegerJSON := jaegerJSONSpanUnmarshaler{}
	zipkinProto := newPdataTracesUnmarshaler(zipkinv2.NewProtobufTracesUnmarshaler(false, false), "zipkin_proto")
	zipkinJSON := newPdataTracesUnmarshaler(zipkinv2.NewJSONTracesUnmarshaler(false), "zipkin_json")
	zipkinThrift := newPdataTracesUnmarshaler(zipkinv1.NewThriftTracesUnmarshaler(), "zipkin_thrift")
	return map[string]TracesUnmarshaler{
		otlpPb.Encoding():       otlpPb,
		otlpJSON.Encoding():     otlpJSON,
		jaegerProto.Encoding():  jaegerProto,
		jaegerJSON.Encoding():   jaegerJSON,
		zipkinProto.Encoding():  zipkinProto,
		zipkinJSON.Encoding():   zipkinJSON,
		zipkinThrift.Encoding(): zipkinThrift,
	}
}

func defaultMetricsUnmarshalers() map[string]MetricsUnmarshaler {
	otlpPb := newPdataMetricsUnmarshaler(&pmetric.ProtoUnmarshaler{}, defaultEncoding)
	otlpJSON := newPdataMetricsUnmarshaler(&pmetric.JSONUnmarshaler{}, "otlp_json")
	return map[string]MetricsUnmarshaler{
		otlpPb.Encoding():   otlpPb,
		otlpJSON.Encoding(): otlpJSON,
	}
}

func defaultLogsUnmarshalers() map[string]LogsUnmarshaler {
	otlpPb := newPdataLogsUnmarshaler(&plog.ProtoUnmarshaler{}, defaultEncoding)
	otlpJSON := newPdataLogsUnmarshaler(&plog.JSONUnmarshaler{}, "otlp_json")
	return map[string]LogsUnmarshaler{
		otlpPb.Encoding():   otlpPb,
		otlpJSON.Encoding(): otlpJSON,
	}
}
