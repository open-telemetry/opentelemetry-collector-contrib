// Copyright 2020 OpenTelemetry Authors
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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap/zaptest"
)

type producerMock struct {
	mock.Mock
}

func (m *producerMock) start() {
	m.Called()
}

func (m *producerMock) stop() {
	m.Called()
}

func (m *producerMock) put(data []byte, partitionKey string) error {
	args := m.Called(data, partitionKey)
	return args.Error(0)
}

type marshallerMock struct {
	Marshaller
	mock.Mock
}

func (m *marshallerMock) MarshalTraces(td pdata.Traces) ([]Message, error) {
	args := m.Called(td)
	return args.Get(0).([]Message), args.Error(1)
}

func TestNewKinesisExporter(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, err := newExporter(cfg, zaptest.NewLogger(t))
	assert.NotNil(t, exp)
	assert.NoError(t, err)
}

func TestPushingTracesToKinesisQueue(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, _ := newExporter(cfg, zaptest.NewLogger(t))
	mockProducer := new(producerMock)
	exp.producer = mockProducer
	require.NotNil(t, exp)

	mockProducer.On("put", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	dropped, err := exp.pushTraces(context.Background(), pdata.NewTraces())
	assert.NoError(t, err)
	assert.Equal(t, 0, dropped)
}

func TestPushingTracesNilData(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, _ := newExporter(cfg, zaptest.NewLogger(t))
	mockMarshaller := new(marshallerMock)
	exp.marshaller = mockMarshaller
	require.NotNil(t, exp)

	mockMarshaller.On("MarshalTraces", mock.AnythingOfType("pdata.Traces")).Return([]Message(nil), errors.New(""))
	spanCount := 2
	trace := generateEmptyTrace(spanCount)
	dropped, err := exp.pushTraces(context.Background(), trace)
	assert.Error(t, err)
	assert.Equal(t, trace.SpanCount(), dropped)
}

func TestPushingSomeBadSpans(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	exp, _ := newExporter(cfg, zaptest.NewLogger(t))
	mockMarshaller := new(marshallerMock)
	exp.marshaller = mockMarshaller
	require.NotNil(t, exp)

	spanCount := 1
	data, err := generateEmptyTrace(spanCount).ToOtlpProtoBytes()
	require.NoError(t, err)

	returnMsgs := []Message{{Value: nil}, {Value: data}, {Value: nil}}
	expectedSpansDropped := 2
	mockMarshaller.On("MarshalTraces", mock.AnythingOfType("pdata.Traces")).Return(returnMsgs, errors.New(""))

	dropped, err := exp.pushTraces(context.Background(), pdata.NewTraces())
	assert.Error(t, err)
	assert.Equal(t, expectedSpansDropped, dropped)
}

func generateEmptyTrace(spanCount int) pdata.Traces {
	td := pdata.NewTraces()
	spans := createSpanSlice(td)
	spans.Resize(spanCount)
	for i := 0; i < spanCount; i++ {
		spans.At(i).InitEmpty()
	}
	return td
}

func generateValidTrace(spanCount int) pdata.Traces {
	td := pdata.NewTraces()
	spans := createSpanSlice(td)
	spans.Resize(spanCount)
	for i := 0; i < spanCount; i++ {
		span := spans.At(i)
		span.SetName("foo")
		span.SetStartTime(pdata.TimestampUnixNano(10))
		span.SetEndTime(pdata.TimestampUnixNano(20))
		span.SetTraceID(pdata.NewTraceID([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		span.SetSpanID(pdata.NewSpanID([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	}

	return td
}

func createSpanSlice(td pdata.Traces) pdata.SpanSlice {
	td.ResourceSpans().Resize(1)
	ils := td.ResourceSpans().At(0).InstrumentationLibrarySpans()
	ils.Resize(1)
	spans := ils.At(0).Spans()
	return spans
}
