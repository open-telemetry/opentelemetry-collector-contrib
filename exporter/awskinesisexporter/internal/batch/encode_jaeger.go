// Copyright  OpenTelemetry Authors
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

package batch

import (
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaeger struct {
	batchSize  int
	recordSize int
}

var _ Encoder = (*jaeger)(nil)

func NewJaeger(batchSize, recordSize int) Encoder {
	return jaeger{
		batchSize:  batchSize,
		recordSize: recordSize,
	}
}

func (j jaeger) Traces(td pdata.Traces) (*Batch, error) {
	traces, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		return nil, err
	}

	bt := New(
		WithMaxRecordSize(j.recordSize),
		WithMaxRecordsPerBatch(j.batchSize),
	)
	var errs []error
	for _, trace := range traces {
		for _, span := range trace.GetSpans() {
			if span.Process == nil {
				span.Process = trace.Process
			}
			if err := bt.AddProtobufV1(span, span.TraceID.String()); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return bt, consumererror.Combine(errs)
}

func (jaeger) Metrics(_ pdata.Metrics) (*Batch, error) {
	return nil, ErrUnsupportedEncodedType
}

func (jaeger) Logs(_ pdata.Logs) (*Batch, error) {
	return nil, ErrUnsupportedEncodedType
}
