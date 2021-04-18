// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

// TracesMarshaller marshals traces into Message array.
type TracesMarshaller interface {
	// Marshal serializes spans into Messages
	Marshal(traces pdata.Traces) ([]Message, error)

	// Encoding returns encoding name
	Encoding() string
}

// MetricsMarshaller marshals metrics into Message array
type MetricsMarshaller interface {
	// Marshal serializes metrics into Messages
	Marshal(metrics pdata.Metrics) ([]Message, error)

	// Encoding returns encoding name
	Encoding() string
}

// LogsMarshaller marshals logs into Message array
type LogsMarshaller interface {
	// Marshal serializes logs into Messages
	Marshal(logs pdata.Logs) ([]Message, error)

	// Encoding returns encoding name
	Encoding() string
}

// Message encapsulates Pulsar's message payload.
type Message struct {
	Value []byte
	Key   []byte
}

// tracesMarshallers returns map of supported encodings with TracesMarshaller.
func tracesMarshallers() map[string]TracesMarshaller {
	otlppb := &otlpTracesPbMarshaller{}
	return map[string]TracesMarshaller{
		otlppb.Encoding(): otlppb,
	}
}

// metricsMarshallers returns map of supported encodings and MetricsMarshaller
func metricsMarshallers() map[string]MetricsMarshaller {
	otlppb := &otlpMetricsPbMarshaller{}
	return map[string]MetricsMarshaller{
		otlppb.Encoding(): otlppb,
	}
}

// logsMarshallers returns map of supported encodings and LogsMarshaller
func logsMarshallers() map[string]LogsMarshaller {
	otlppb := &otlpLogsPbMarshaller{}
	return map[string]LogsMarshaller{
		otlppb.Encoding(): otlppb,
	}
}
