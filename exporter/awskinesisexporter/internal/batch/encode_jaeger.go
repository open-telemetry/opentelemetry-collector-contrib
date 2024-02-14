// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"github.com/gogo/protobuf/proto"
	"github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func partitionByTraceID(v any) string {
	if s, ok := v.(*model.Span); ok && s != nil {
		return s.TraceID.String()
	}
	return key.Randomized(v)
}

type jaegerEncoder struct {
	batchOptions []Option
}

var _ Encoder = (*jaegerEncoder)(nil)

func (je jaegerEncoder) Traces(td ptrace.Traces) (*Batch, error) {
	traces, err := jaeger.ProtoFromTraces(td)
	if err != nil {
		return nil, consumererror.NewTraces(err, td)
	}

	bt := New(je.batchOptions...)

	var errs error
	for _, trace := range traces {
		for _, span := range trace.GetSpans() {
			if span.Process == nil {
				span.Process = trace.GetProcess()
			}
			data, err := proto.Marshal(span)
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}
			errs = multierr.Append(errs, bt.AddRecord(data, partitionByTraceID(span)))
		}
	}

	return bt, errs
}

func (jaegerEncoder) Logs(plog.Logs) (*Batch, error)          { return nil, ErrUnsupportedEncoding }
func (jaegerEncoder) Metrics(pmetric.Metrics) (*Batch, error) { return nil, ErrUnsupportedEncoding }
