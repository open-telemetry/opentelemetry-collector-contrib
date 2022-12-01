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

package kafkaexporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// data is a simple means of allowing
// interchangeability between the
// different marshaller types
type data interface {
	ptrace.Traces | plog.Logs | pmetric.Metrics
}

type mockMarshaler[Data data] struct {
	consume  func(d Data, topic string) ([]*sarama.ProducerMessage, error)
	encoding string
}

func (mm *mockMarshaler[Data]) Encoding() string { return mm.encoding }

func (mm *mockMarshaler[Data]) Marshal(d Data, topic string) ([]*sarama.ProducerMessage, error) {
	if mm.consume != nil {
		return mm.consume(d, topic)
	}
	return nil, errors.New("not implemented")
}

func newMockMarshaler[Data data](encoding string) *mockMarshaler[Data] {
	return &mockMarshaler[Data]{
		encoding: encoding,
		consume: func(d Data, topic string) ([]*sarama.ProducerMessage, error) {
			return []*sarama.ProducerMessage{
				{
					Topic: topic,
					// message payload will simply be the marshaler's encoding to easily confirm a specific marshaler was used
					Value: sarama.StringEncoder(encoding),
				},
			}, nil
		},
	}
}

// applyConfigOption is used to modify values of the
// the default exporter config to make it easier to
// use the return in a test table set up
func applyConfigOption(option func(conf *Config)) *Config {
	conf := createDefaultConfig().(*Config)
	option(conf)
	return conf
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, []string{defaultBroker}, cfg.Brokers)
	assert.Equal(t, "", cfg.Topic)
}

// convenience method for the TestCreate*Exporter tests
// sets up a mock producer which validates the message payload sent by the exporter
func mockProducerFunc(t *testing.T, expectedValue []byte) ProducerFactoryFunc {
	// setup producer mock which validates message was encoded by the configured marshaler
	producer := mocks.NewSyncProducer(t, nil)
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
		valueBytes, err := pm.Value.Encode()
		if err != nil {
			return err
		}
		if !bytes.Equal(expectedValue, valueBytes) {
			return fmt.Errorf("unexpected payload (%s) for encoding: %s", string(valueBytes), expectedValue)
		}
		return nil
	})
	return func(config Config) (sarama.SyncProducer, error) { return producer, nil }
}

func TestCreateMetricExporter(t *testing.T) {
	t.Parallel()

	// Fake metrics to export
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")

	// default encoding of above metrics.
	protoMetrics, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(metrics)
	assert.NoError(t, err, "Expect no error marshaling hard-coded pmetric.Metrics")

	tests := []struct {
		name         string
		conf         *Config
		marshalers   []MetricsMarshaler
		producerFunc ProducerFactoryFunc
		err          error
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				// this disables contacting the broker so
				// we can successfully create the exporter
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          nil,
		},
		{
			name: "invalid config (validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
			}),
			marshalers: nil,
			// oltp_encoding
			producerFunc: mockProducerFunc(t, protoMetrics),
			err:          nil,
		},
		{
			name: "custom_encoding",
			conf: applyConfigOption(func(conf *Config) {
				conf.Encoding = "custom"
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
			}),
			marshalers: []MetricsMarshaler{
				newMockMarshaler[pmetric.Metrics]("custom"),
			},
			producerFunc: mockProducerFunc(t, []byte("custom")),
			err:          nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory(WithMetricsMarshalers(tc.marshalers...), WithProducerFactory(tc.producerFunc))
			exporter, err := f.CreateMetricsExporter(
				context.Background(),
				componenttest.NewNopExporterCreateSettings(),
				tc.conf,
			)
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				assert.Nil(t, exporter, "Must return nil value for invalid exporter")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")

			// sends some metrics to the exporter to see if we get the expected message for the configuration
			err = exporter.ConsumeMetrics(context.TODO(), metrics)
			assert.NoError(t, err, "exporter should not error if the expected message is produced")
		})
	}
}

func TestCreateLogExporter(t *testing.T) {
	t.Parallel()

	// Fake logs to export
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test")

	// default encoding of above logs.
	protoLogs, err := (&plog.ProtoMarshaler{}).MarshalLogs(logs)
	assert.NoError(t, err, "Expect no error marshaling hard-coded plog.Logs")

	tests := []struct {
		name         string
		conf         *Config
		marshalers   []LogsMarshaler
		producerFunc ProducerFactoryFunc
		err          error
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				// this disables contacting the broker so
				// we can successfully create the exporter
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          nil,
		},
		{
			name: "invalid config (validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
				conf.Encoding = defaultEncoding
			}),
			marshalers:   nil,
			producerFunc: mockProducerFunc(t, protoLogs),
			err:          nil,
		},
		{
			name: "custom_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
				conf.Encoding = "custom"
			}),
			producerFunc: mockProducerFunc(t, []byte("custom")),
			marshalers: []LogsMarshaler{
				newMockMarshaler[plog.Logs]("custom"),
			},
			err: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory(WithLogsMarshalers(tc.marshalers...), WithProducerFactory(tc.producerFunc))
			exporter, err := f.CreateLogsExporter(
				context.Background(),
				componenttest.NewNopExporterCreateSettings(),
				tc.conf,
			)
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				assert.Nil(t, exporter, "Must return nil value for invalid exporter")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")

			// sends some logs to the exporter to see if we get the expected message for the configuration
			err = exporter.ConsumeLogs(context.TODO(), logs)
			assert.NoError(t, err, "exporter should not error if the expected message is produced")
		})
	}
}

func TestCreateTraceExporter(t *testing.T) {
	t.Parallel()

	// fake traces for export
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("testSpan")

	// default encoding of above logs.
	protoTraces, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	assert.NoError(t, err, "Expect no error marshaling hard-coded ptrace.Traces")

	tests := []struct {
		name         string
		conf         *Config
		marshalers   []TracesMarshaler
		producerFunc ProducerFactoryFunc
		err          error
	}{
		{
			name: "valid config (no validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          nil,
		},
		{
			name: "invalid config (validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers:   nil,
			producerFunc: newSaramaProducer,
			err:          &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
				conf.Encoding = defaultEncoding
			}),
			marshalers:   nil,
			producerFunc: mockProducerFunc(t, protoTraces),
			err:          nil,
		},
		{
			name: "custom_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// disabling queue for exporter helper so metrics go right to pusher
				conf.QueueSettings = exporterhelper.QueueSettings{
					Enabled: false,
				}
				conf.RetrySettings = exporterhelper.RetrySettings{
					Enabled: false,
				}
				conf.Encoding = "custom"
			}),
			marshalers: []TracesMarshaler{
				newMockMarshaler[ptrace.Traces]("custom"),
			},
			producerFunc: mockProducerFunc(t, []byte("custom")),
			err:          nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory(WithTracesMarshalers(tc.marshalers...), WithProducerFactory(tc.producerFunc))
			exporter, err := f.CreateTracesExporter(
				context.Background(),
				componenttest.NewNopExporterCreateSettings(),
				tc.conf,
			)
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				assert.Nil(t, exporter, "Must return nil value for invalid exporter")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")

			// sends some traces to the exporter to see if we get the expected message for the configuration
			err = exporter.ConsumeTraces(context.TODO(), traces)
			assert.NoError(t, err, "exporter should not error if the expected message is produced")
		})
	}
}
