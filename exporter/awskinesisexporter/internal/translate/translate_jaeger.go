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

package translate

import (
	awskinesis "github.com/signalfx/opencensus-go-exporter-kinesis"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaeger struct {
	kinesis *awskinesis.Exporter
}

// Ensure the jaeger encoder meets the interface at compile time.
var _ ExportWriter = (*jaeger)(nil)

func JaegerExporter(kinesis *awskinesis.Exporter) ExportWriter {
	return &jaeger{kinesis: kinesis}
}

func (j *jaeger) WriteTraces(td pdata.Traces) error {
	traces, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		return err
	}

	var errs []error
	for _, trace := range traces {
		for _, span := range trace.GetSpans() {
			if span.Process == nil {
				span.Process = trace.Process
			}
			if err := j.kinesis.ExportSpan(span); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return consumererror.Combine(errs)
}

func (j *jaeger) WriteMetrics(_ pdata.Metrics) error { return ErrUnsupportedEncodedType }
func (j *jaeger) WriteLogs(_ pdata.Logs) error       { return ErrUnsupportedEncodedType }
