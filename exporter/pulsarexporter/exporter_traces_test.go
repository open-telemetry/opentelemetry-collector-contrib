// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"context"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func Test_tracerPublisher(t *testing.T) {
	mProducer := &mockProducer{name: "producer1", topic: "default"}
	producer := pulsarTracesProducer{client: nil, producer: mProducer, marshaler: tracesMarshalers()["jaeger_proto"]}
	err := producer.tracesPusher(context.Background(), testdata.GenerateTracesManySpansSameResource(10))

	assert.NoError(t, err)
}

func Test_tracerPublisher_marshaler_err(t *testing.T) {
	mProducer := &mockProducer{name: "producer1", topic: "default"}
	producer := pulsarTracesProducer{client: nil, producer: mProducer, marshaler: &customTraceMarshaler{encoding: "unknown"}}
	err := producer.tracesPusher(context.Background(), testdata.GenerateTracesManySpansSameResource(10))

	assert.NotNil(t, err)
	assert.True(t, consumererror.IsPermanent(err))
}

var _ pulsar.Producer = (*mockProducer)(nil)

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
