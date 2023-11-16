// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type mockHost struct {
	component.Host
	ext map[component.ID]component.Component
}

func (mh mockHost) GetExtensions() map[component.ID]component.Component {
	return mh.ext
}

type mockPulsarConsumer struct {
	messageChan chan pulsar.ConsumerMessage
}

func (c mockPulsarConsumer) Subscription() string {
	panic("implement me")
}

func (c mockPulsarConsumer) Unsubscribe() error {
	panic("implement me")
}

func (c mockPulsarConsumer) Receive(_ context.Context) (pulsar.Message, error) {
	msg, ok := <-c.messageChan
	if !ok {
		return nil, errors.New(alreadyClosedError)
	}
	return msg, nil
}

func (c mockPulsarConsumer) Chan() <-chan pulsar.ConsumerMessage {
	return c.messageChan
}

func (c mockPulsarConsumer) Ack(_ pulsar.Message) {
	// no-op
}

func (c mockPulsarConsumer) ReconsumeLater(_ pulsar.Message, _ time.Duration) {
	panic("implement me")
}

func (c mockPulsarConsumer) AckID(_ pulsar.MessageID) {
	panic("implement me")
}

func (c mockPulsarConsumer) Nack(_ pulsar.Message) {
	// no-op
}

func (c mockPulsarConsumer) NackID(_ pulsar.MessageID) {
	panic("implement me")
}

func (c mockPulsarConsumer) Close() {}

func (c mockPulsarConsumer) Seek(_ pulsar.MessageID) error {
	panic("implement me")
}

func (c mockPulsarConsumer) SeekByTime(_ time.Time) error {
	panic("implement me")
}

func (c mockPulsarConsumer) Name() string {
	panic("implement me")
}

type mockPulsarMessage struct {
	payload []byte
}

func (msg mockPulsarMessage) Topic() string {
	panic("implement me")
}

func (msg mockPulsarMessage) Properties() map[string]string {
	panic("implement me")
}

func (msg mockPulsarMessage) Payload() []byte {
	return msg.payload
}

func (msg mockPulsarMessage) ID() pulsar.MessageID {
	panic("implement me")
}

func (msg mockPulsarMessage) PublishTime() time.Time {
	panic("implement me")
}

func (msg mockPulsarMessage) EventTime() time.Time {
	panic("implement me")
}

func (msg mockPulsarMessage) Key() string {
	panic("implement me")
}

func (msg mockPulsarMessage) OrderingKey() string {
	panic("implement me")
}

func (msg mockPulsarMessage) RedeliveryCount() uint32 {
	panic("implement me")
}

func (msg mockPulsarMessage) IsReplicated() bool {
	panic("implement me")
}

func (msg mockPulsarMessage) GetReplicatedFrom() string {
	panic("implement me")
}

func (msg mockPulsarMessage) GetSchemaValue(_ interface{}) error {
	panic("implement me")
}

func (msg mockPulsarMessage) ProducerName() string {
	panic("implement me")
}

func (msg mockPulsarMessage) GetEncryptionContext() *pulsar.EncryptionContext {
	panic("implement me")
}

type mockPulsarClient struct{}

func (_ mockPulsarClient) CreateProducer(pulsar.ProducerOptions) (pulsar.Producer, error) {
	panic("implement me")
}

func (_ mockPulsarClient) Subscribe(pulsar.ConsumerOptions) (pulsar.Consumer, error) {
	return mockPulsarConsumer{}, nil
}

func (_ mockPulsarClient) CreateReader(pulsar.ReaderOptions) (pulsar.Reader, error) {
	panic("implement me")
}

func (_ mockPulsarClient) TopicPartitions(topic string) ([]string, error) {
	panic("implement me")
}

func (_ mockPulsarClient) Close() {}

func TestTracesReceiverCreate(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), factory.CreateDefaultConfig(), nil)
	assert.Nil(t, err)
}

func TestTracesReceiverEncoding(t *testing.T) {
	tests := []struct {
		name         string
		id           component.ID
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "otlp",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "jaeger",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "zipkin",
			id:   component.NewIDWithName(component.DataTypeTraces, "zipkin"),
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "jsonlog",
			id:   component.NewIDWithName(component.DataTypeTraces, "jsonlog"),
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement TracesUnmarshaler",
		},
		{
			name: "text",
			id:   component.NewIDWithName(component.DataTypeTraces, "text"),
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement TracesUnmarshaler",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			assert.Nil(t, err)
			host := mockHost{
				ext: map[component.ID]component.Component{
					test.id: ext,
				},
			}
			receiver := &pulsarTracesConsumer{
				tracesConsumer: nil,
				encoding:       &test.id,
				settings:       receivertest.NewNopCreateSettings(),
				client:         mockPulsarClient{},
			}
			if test.expectedErr != "" {
				assert.ErrorContains(t, receiver.Start(context.Background(), host), test.expectedErr)
			} else {
				assert.NoError(t, receiver.Start(context.Background(), host))
			}
		})
	}
}

func TestMetricsReceiverCreate(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), factory.CreateDefaultConfig(), nil)
	assert.Nil(t, err)
}

func TestMetricsReceiverEncoding(t *testing.T) {
	tests := []struct {
		name         string
		id           component.ID
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "otlp",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "jaeger",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement MetricsUnmarshaler",
		},
		{
			name: "zipkin",
			id:   component.NewIDWithName(component.DataTypeTraces, "zipkin"),
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement MetricsUnmarshaler",
		},
		{
			name: "jsonlog",
			id:   component.NewIDWithName(component.DataTypeTraces, "jsonlog"),
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement MetricsUnmarshaler",
		},
		{
			name: "text",
			id:   component.NewIDWithName(component.DataTypeTraces, "text"),
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement MetricsUnmarshaler",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			assert.Nil(t, err)
			host := mockHost{
				ext: map[component.ID]component.Component{
					test.id: ext,
				},
			}
			receiver := &pulsarMetricsConsumer{
				metricsConsumer: nil,
				encoding:        &test.id,
				settings:        receivertest.NewNopCreateSettings(),
				client:          mockPulsarClient{},
			}
			if test.expectedErr != "" {
				assert.ErrorContains(t, receiver.Start(context.Background(), host), test.expectedErr)
			} else {
				assert.NoError(t, receiver.Start(context.Background(), host))
			}
		})
	}
}

func TestLogsReceiverCreate(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), factory.CreateDefaultConfig(), nil)
	assert.Nil(t, err)
}

func TestLogsReceiverEncoding(t *testing.T) {
	tests := []struct {
		name         string
		id           component.ID
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "otlp",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := otlpencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "jaeger",
			id:   component.NewIDWithName(component.DataTypeTraces, "otlp"),
			getExtension: func() (extension.Extension, error) {
				factory := jaegerencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement LogsUnmarshaler",
		},
		{
			name: "zipkin",
			id:   component.NewIDWithName(component.DataTypeTraces, "zipkin"),
			getExtension: func() (extension.Extension, error) {
				factory := zipkinencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
			expectedErr: "doesn't implement LogsUnmarshaler",
		},
		{
			name: "jsonlog",
			id:   component.NewIDWithName(component.DataTypeTraces, "jsonlog"),
			getExtension: func() (extension.Extension, error) {
				factory := jsonlogencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "text",
			id:   component.NewIDWithName(component.DataTypeTraces, "text"),
			getExtension: func() (extension.Extension, error) {
				factory := textencodingextension.NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			assert.Nil(t, err)
			host := mockHost{
				ext: map[component.ID]component.Component{
					test.id: ext,
				},
			}
			receiver := &pulsarLogsConsumer{
				logsConsumer: nil,
				encoding:     &test.id,
				settings:     receivertest.NewNopCreateSettings(),
				client:       mockPulsarClient{},
			}
			if test.expectedErr != "" {
				assert.ErrorContains(t, receiver.Start(context.Background(), host), test.expectedErr)
			} else {
				assert.NoError(t, receiver.Start(context.Background(), host))
			}
		})
	}
}
