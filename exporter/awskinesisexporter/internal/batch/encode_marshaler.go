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
	"errors"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
)

type batchMarshaller struct {
	batchOptions []Option
	partitioner  key.Partition

	logsMarshaller    plog.Marshaler
	tracesMarshaller  ptrace.Marshaler
	metricsMarshaller pmetric.Marshaler
}

var _ Encoder = (*batchMarshaller)(nil)

func (bm *batchMarshaller) Logs(ld plog.Logs) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := plog.NewLogs()
	export.ResourceLogs().AppendEmpty()

	var errs error
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		line := ld.ResourceLogs().At(i)
		line.CopyTo(export.ResourceLogs().At(0))

		data, err := bm.logsMarshaller.MarshalLogs(export)
		if err != nil {
			if errors.Is(err, ErrUnsupportedEncoding) {
				return nil, err
			}
			errs = multierr.Append(errs, consumererror.NewLogs(err, export))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(export)); err != nil {
			errs = multierr.Append(errs, consumererror.NewLogs(err, export))
		}
	}

	return bt, errs
}

func (bm *batchMarshaller) Traces(td ptrace.Traces) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := ptrace.NewTraces()
	export.ResourceSpans().AppendEmpty()

	var errs error
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		span := td.ResourceSpans().At(i)
		span.CopyTo(export.ResourceSpans().At(0))

		data, err := bm.tracesMarshaller.MarshalTraces(export)
		if err != nil {
			if errors.Is(err, ErrUnsupportedEncoding) {
				return nil, err
			}
			errs = multierr.Append(errs, consumererror.NewTraces(err, export))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(span)); err != nil {
			errs = multierr.Append(errs, consumererror.NewTraces(err, export))
		}
	}

	return bt, errs
}

func (bm *batchMarshaller) Metrics(md pmetric.Metrics) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := pmetric.NewMetrics()
	export.ResourceMetrics().AppendEmpty()

	var errs error
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		datapoint := md.ResourceMetrics().At(i)
		datapoint.CopyTo(export.ResourceMetrics().At(0))

		data, err := bm.metricsMarshaller.MarshalMetrics(export)
		if err != nil {
			if errors.Is(err, ErrUnsupportedEncoding) {
				return nil, err
			}
			errs = multierr.Append(errs, consumererror.NewMetrics(err, export))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(export)); err != nil {
			errs = multierr.Append(errs, consumererror.NewMetrics(err, export))
		}
	}

	return bt, errs
}
