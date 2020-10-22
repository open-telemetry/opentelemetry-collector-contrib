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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
)

func TestCorrectJaegerEncoding(t *testing.T) {
	t.Parallel()
	m := otlpProtoMarshaller{}
	assert.Equal(t, otlpProto, m.Encoding())
}

func TestJaegerMarshaller(t *testing.T) {
	t.Parallel()
	td := generateValidTrace(2)

	batches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	var expectedMsgs []Message
	for _, batch := range batches {
		for _, span := range batch.Spans {
			msg, marshalErr := span.Marshal()
			require.NoError(t, marshalErr)
			expectedMsgs = append(expectedMsgs, Message{Value: msg})
		}
	}

	require.NoError(t, err)
	require.NotEmpty(t, expectedMsgs)

	m := jaegerMarshaller{}
	bytes, err := m.MarshalTraces(td)
	assert.NoError(t, err)
	assert.Equal(t, expectedMsgs, bytes)
}

func TestJaegerMarshallerErrorInvalidTraces(t *testing.T) {
	t.Parallel()
	td := generateEmptyTrace(1)

	m := jaegerMarshaller{}
	msgs, err := m.MarshalTraces(td)
	assert.Error(t, err)
	assert.Nil(t, msgs)
}
