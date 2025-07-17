// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

var (
	// ErrUnsupportedEncoding is used when the encoder type does not support the type of encoding
	ErrUnsupportedEncoding = errors.New("unsupported type to encode")
	// ErrUnknownExportEncoder is used when a named encoding doesn't not exist
	ErrUnknownExportEncoder = errors.New("unknown encoding export format")
)

// Encoder transforms the internal pipeline format into a configurable
// format that is then used to export to kinesis.
type Encoder interface {
	Metrics(md pmetric.Metrics) (*Batch, error)

	Traces(td ptrace.Traces) (*Batch, error)

	Logs(ld plog.Logs) (*Batch, error)
}

func NewEncoder(named string, batchOptions ...Option) (Encoder, error) {
	bm := &batchMarshaller{
		batchOptions:      batchOptions,
		partitioner:       key.Randomized,
		logsMarshaller:    unsupported{},
		tracesMarshaller:  unsupported{},
		metricsMarshaller: unsupported{},
	}
	switch named {
	case "zipkin_proto":
		bm.tracesMarshaller = zipkinv2.NewProtobufTracesMarshaler()
	case "zipkin_json":
		bm.tracesMarshaller = zipkinv2.NewJSONTracesMarshaler()
	case "otlp", "otlp_proto":
		bm.logsMarshaller = &plog.ProtoMarshaler{}
		bm.metricsMarshaller = &pmetric.ProtoMarshaler{}
		bm.tracesMarshaller = &ptrace.ProtoMarshaler{}
	case "otlp_json":
		bm.logsMarshaller = &plog.JSONMarshaler{}
		bm.metricsMarshaller = &pmetric.JSONMarshaler{}
		bm.tracesMarshaller = &ptrace.JSONMarshaler{}
	case "jaeger_proto":
		// Jaeger encoding is a special case
		// since the internal libraries offer no means of ptrace.TraceMarshaller.
		// In order to preserve historical behavior, a custom type
		// is used until it can be replaced.
		return &jaegerEncoder{
			batchOptions: batchOptions,
		}, nil
	default:
		return nil, ErrUnknownExportEncoder
	}
	return bm, nil
}
