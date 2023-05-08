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

package pulsarexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNewMetricsExporter_err_encoding(t *testing.T) {
	c := Config{Encoding: "bar"}
	mexp, err := newMetricsExporter(c, exportertest.NewNopCreateSettings(), metricsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)
}

func TestNewMetricsExporter_err_traces_encoding(t *testing.T) {
	c := Config{Encoding: "jaeger_proto"}
	mexp, err := newMetricsExporter(c, exportertest.NewNopCreateSettings(), metricsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)

}

func TestNewLogsExporter_err_encoding(t *testing.T) {
	c := Config{Encoding: "bar"}
	mexp, err := newLogsExporter(c, exportertest.NewNopCreateSettings(), logsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)
}

func TestNewLogsExporter_err_traces_encoding(t *testing.T) {
	c := Config{Encoding: "jaeger_proto"}
	mexp, err := newLogsExporter(c, exportertest.NewNopCreateSettings(), logsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)
}

func Test_tracerPublisher(t *testing.T) {
	mProducer := &mockProducer{name: "producer1", topic: "default"}
	producer := PulsarTracesProducer{client: nil, producer: mProducer, marshaler: tracesMarshalers()["jaeger_proto"]}
	err := producer.tracesPusher(context.Background(), testdata.GenerateTracesManySpansSameResource(10))

	assert.NoError(t, err)
}

func Test_tracerPublisher_marshaler_err(t *testing.T) {
	mProducer := &mockProducer{name: "producer1", topic: "default"}
	producer := PulsarTracesProducer{client: nil, producer: mProducer, marshaler: &customTraceMarshaler{encoding: "unknown"}}
	err := producer.tracesPusher(context.Background(), testdata.GenerateTracesManySpansSameResource(10))

	assert.NotNil(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

type customTraceMarshaler struct {
	encoding string
}

func (c *customTraceMarshaler) Marshal(ptrace.Traces, string) ([]*pulsar.ProducerMessage, error) {
	return nil, errors.New("unsupported encoding")
}

func (c *customTraceMarshaler) Encoding() string {
	return c.encoding
}

type mockProducer struct {
	topic string
	name  string
}

func (c *mockProducer) Topic() string {
	return c.topic
}

func (c *mockProducer) Name() string {
	return c.name
}

func (c *mockProducer) Send(context.Context, *pulsar.ProducerMessage) (pulsar.MessageID, error) {
	return nil, nil
}

func (c *mockProducer) SendAsync(context.Context, *pulsar.ProducerMessage, func(pulsar.MessageID, *pulsar.ProducerMessage, error)) {

}

func (c *mockProducer) LastSequenceID() int64 {
	return 1
}

func (c *mockProducer) Flush() error {
	return nil
}

func (c *mockProducer) Close() {
}
