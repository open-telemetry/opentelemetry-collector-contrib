// Copyright 2020 The OpenTelemetry Authors
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

package kinesisexporter

import (
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
)

type jaegerMarshaller struct{}

var _ Marshaller = (*jaegerMarshaller)(nil)

func (*jaegerMarshaller) MarshalTraces(traces pdata.Traces) ([]Message, error) {
	pBatches, err := jaegertranslator.InternalTracesToJaegerProto(traces)
	if err != nil {
		return nil, err
	}

	var spanData []Message
	var errors []error
	for _, batch := range pBatches {
		for _, span := range batch.GetSpans() {
			data, err := span.Marshal()
			if err != nil {
				errors = append(errors, err)
			}
			// Append data regardless so we can count number of failed spans in export function
			spanData = append(spanData, Message{Value: data})
		}
	}

	return spanData, componenterror.CombineErrors(errors)
}

func (*jaegerMarshaller) Encoding() string {
	return "jaeger_proto"
}
