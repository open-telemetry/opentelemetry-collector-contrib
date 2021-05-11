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

package awskinesisexporter

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

type otlpProtoMarshaller struct{}

var _ Marshaller = (*otlpProtoMarshaller)(nil)

func (*otlpProtoMarshaller) MarshalTraces(traces pdata.Traces) ([]byte, error) {
	return traces.ToOtlpProtoBytes()
}

func (*otlpProtoMarshaller) MarshalMetrics(metrics pdata.Metrics) ([]byte, error) {
	return metrics.ToOtlpProtoBytes()
}

func (*otlpProtoMarshaller) Encoding() string {
	return otlpProto
}
