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

	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"

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
	Metrics(md pdata.Metrics) (*Batch, error)

	Traces(td pdata.Traces) (*Batch, error)

	Logs(ld pdata.Logs) (*Batch, error)
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
		bm.logsMarshaller = otlp.NewProtobufLogsMarshaler()
		bm.metricsMarshaller = otlp.NewProtobufMetricsMarshaler()
		bm.tracesMarshaller = otlp.NewProtobufTracesMarshaler()
	case "otlp_json":
		bm.logsMarshaller = otlp.NewJSONLogsMarshaler()
		bm.metricsMarshaller = otlp.NewJSONMetricsMarshaler()
		bm.tracesMarshaller = otlp.NewJSONTracesMarshaler()
	case "jaeger_proto":
		// Jaeger encoding is a special case
		// since the internal libraries offer no means of pdata.TraceMarshaller.
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
