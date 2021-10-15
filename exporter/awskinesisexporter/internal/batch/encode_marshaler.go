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
	"errors"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
)

type batchMarshaller struct {
	batchOptions []Option
	partitioner  key.Partition

	logsMarshaller    pdata.LogsMarshaler
	tracesMarshaller  pdata.TracesMarshaler
	metricsMarshaller pdata.MetricsMarshaler
}

var _ Encoder = (*batchMarshaller)(nil)

func (bm *batchMarshaller) Logs(ld pdata.Logs) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := pdata.NewLogs()
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
			errs = multierr.Append(errs, consumererror.NewLogs(err, export.Clone()))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(export)); err != nil {
			errs = multierr.Append(errs, consumererror.NewLogs(err, export.Clone()))
		}
	}

	return bt, errs
}

func (bm *batchMarshaller) Traces(td pdata.Traces) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := pdata.NewTraces()
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
			errs = multierr.Append(errs, consumererror.NewTraces(err, export.Clone()))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(span)); err != nil {
			errs = multierr.Append(errs, consumererror.NewTraces(err, export.Clone()))
		}
	}

	return bt, errs
}

func (bm *batchMarshaller) Metrics(md pdata.Metrics) (*Batch, error) {
	bt := New(bm.batchOptions...)

	// Due to kinesis limitations of only allowing 1Mb of data per record,
	// the resource data is copied to the export variable then marshaled
	// due to no current means of marshaling per resource.

	export := pdata.NewMetrics()
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
			errs = multierr.Append(errs, consumererror.NewMetrics(err, export.Clone()))
			continue
		}

		if err := bt.AddRecord(data, bm.partitioner(export)); err != nil {
			errs = multierr.Append(errs, consumererror.NewMetrics(err, export.Clone()))
		}
	}

	return bt, errs
}
