// Copyright The OpenTelemetry Authors
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

func partitionByTraceID(v interface{}) string {
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
