// Copyright 2020 The OpenTelemetry Authors
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

package kinesisexporter

import (
	"go.opentelemetry.io/collector/consumer/pdata"
)

// Marshaller marshals Opentelemetry data into byte array
type Marshaller interface {
	// MarshalTraces serializes traces into a byte array
	MarshalTraces(traces pdata.Traces) ([]Message, error)
	// Encoding returns the encoding name
	Encoding() string
}

// Message represents a unit of data that is published to kinesis
type Message struct {
	Value []byte
}

// defaultMarshallers returns map of supported encodings with Marshaller.
func defaultMarshallers() map[string]Marshaller {
	return map[string]Marshaller{
		otlpProto:   &otlpProtoMarshaller{},
		jaegerProto: &jaegerMarshaller{},
	}
}
