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

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"errors"
	"fmt"
	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")

const (
	Sync  = "sync"
	Async = "async"
)

type KafkaProducer interface {
	SendMessages(value []*sarama.ProducerMessage) error
	Close() error
}

func KafkaProducerFactory(config Config, logger *zap.Logger) (KafkaProducer, error) {
	if config.SendType == "" {
		config.SendType = Sync
	}
	switch config.SendType {
	case Sync:
		producer, err := NewSyncKafkaProducer(config, logger)
		if err != nil {
			return nil, err
		}
		return producer, nil
	case Async:
		producer, err := NewAsyncKafkaProducer(config, logger)
		if err != nil {
			return nil, err
		}
		return producer, nil
	default:
		logger.Error("kafka SendType error, please choose sync or async")
		return nil, errors.New("kafka SendType error, please choose sync or async")
	}
}

type SyncKafkaProducer struct {
	producer sarama.SyncProducer
	logger   *zap.Logger
}

func NewSyncKafkaProducer(config Config, logger *zap.Logger) (KafkaProducer, error) {
	saramaConf, err := NewSaramaConfig(config)
	if err != nil {
		return nil, err
	}
	syncProducer, err := sarama.NewSyncProducer(config.Brokers, saramaConf)
	if err != nil {
		return nil, err
	}

	kafkaProducer := &SyncKafkaProducer{syncProducer, logger}
	return kafkaProducer, nil
}

func NewAsyncKafkaProducer(config Config, logger *zap.Logger) (KafkaProducer, error) {
	saramaConf, err := NewSaramaConfig(config)
	producer, err := sarama.NewAsyncProducer(config.Brokers, saramaConf)
	if err != nil {
		logger.Error("sarama.NewAsyncProducer err", zap.Error(err))
		return nil, err
	}
	go func(p sarama.AsyncProducer) {
		err := p.Errors()
		success := p.Successes()
		for {
			select {
			case rc := <-err:
				if rc != nil {
					logger.Warn("SendMessages kafka data error", zap.Error(rc.Unwrap()))
				}
			case res := <-success:
				if res != nil {
					logger.Debug("SendMessages kafka data success")
				}
			}
		}
	}(producer)
	kafkaProducer := &AsyncKafkaProducer{producer, logger}
	return kafkaProducer, nil
}

func (k *SyncKafkaProducer) SendMessages(msg []*sarama.ProducerMessage) error {
	err := k.producer.SendMessages(msg)
	if err != nil {
		return err
	}
	return nil
}

func (k *SyncKafkaProducer) Close() error {
	return k.producer.Close()
}

type AsyncKafkaProducer struct {
	producer sarama.AsyncProducer
	logger   *zap.Logger
}

func (k *AsyncKafkaProducer) Close() error {
	return k.producer.Close()
}

func NewSaramaConfig(config Config) (*sarama.Config, error) {
	c := sarama.NewConfig()
	// These setting are required by the sarama implementation.
	c.Producer.Return.Successes = true
	c.Producer.Return.Errors = true
	// Wait only the local commit to succeed before responding.
	c.Producer.RequiredAcks = config.Producer.RequiredAcks
	// Because sarama does not accept a Context for every message, set the Timeout here.
	c.Producer.Timeout = config.Timeout
	c.Metadata.Full = config.Metadata.Full
	c.Metadata.Retry.Max = config.Metadata.Retry.Max
	c.Metadata.Retry.Backoff = config.Metadata.Retry.Backoff
	c.Producer.MaxMessageBytes = config.Producer.MaxMessageBytes
	c.Producer.Flush.MaxMessages = config.Producer.FlushMaxMessages

	if config.ProtocolVersion != "" {
		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		c.Version = version
	}

	if err := ConfigureAuthentication(config.Authentication, c); err != nil {
		return nil, err
	}

	compression, err := saramaProducerCompressionCodec(config.Producer.Compression)
	if err != nil {
		return nil, err
	}
	c.Producer.Compression = compression
	if err := ConfigureAuthentication(config.Authentication, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (k *AsyncKafkaProducer) SendMessages(value []*sarama.ProducerMessage) error {
	k.producer.Input() <- value[0]
	return nil
}

// kafkaTracesProducer uses sarama to produce trace messages to Kafka.
type kafkaTracesProducer struct {
	producer  KafkaProducer
	topic     string
	marshaler TracesMarshaler
	logger    *zap.Logger
}

type kafkaErrors struct {
	count int
	err   string
}

func (ke kafkaErrors) Error() string {
	return fmt.Sprintf("Failed to deliver %d messages due to %s", ke.count, ke.err)
}

func (e *kafkaTracesProducer) tracesPusher(_ context.Context, td ptrace.Traces) error {
	messages, err := e.marshaler.Marshal(td, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	err = e.producer.SendMessages(messages)
	if err != nil {
		var prodErr sarama.ProducerErrors
		if errors.As(err, &prodErr) {
			if len(prodErr) > 0 {
				return kafkaErrors{len(prodErr), prodErr[0].Err.Error()}
			}
		}
		return err
	}
	return nil
}

func (e *kafkaTracesProducer) Close(context.Context) error {
	return e.producer.Close()
}

// kafkaMetricsProducer uses sarama to produce metrics messages to kafka
type kafkaMetricsProducer struct {
	producer  KafkaProducer
	topic     string
	marshaler MetricsMarshaler
	logger    *zap.Logger
}

func (e *kafkaMetricsProducer) metricsDataPusher(_ context.Context, md pmetric.Metrics) error {
	messages, err := e.marshaler.Marshal(md, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	err = e.producer.SendMessages(messages)
	if err != nil {
		var prodErr sarama.ProducerErrors
		if errors.As(err, &prodErr) {
			if len(prodErr) > 0 {
				return kafkaErrors{len(prodErr), prodErr[0].Err.Error()}
			}
		}
		return err
	}
	return nil
}

func (e *kafkaMetricsProducer) Close(context.Context) error {
	return e.producer.Close()
}

// kafkaLogsProducer uses sarama to produce logs messages to kafka
type kafkaLogsProducer struct {
	producer  KafkaProducer
	topic     string
	marshaler LogsMarshaler
	logger    *zap.Logger
}

func (e *kafkaLogsProducer) logsDataPusher(_ context.Context, ld plog.Logs) error {
	messages, err := e.marshaler.Marshal(ld, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	err = e.producer.SendMessages(messages)
	if err != nil {
		var prodErr sarama.ProducerErrors
		if errors.As(err, &prodErr) {
			if len(prodErr) > 0 {
				return kafkaErrors{len(prodErr), prodErr[0].Err.Error()}
			}
		}
		return err
	}
	return nil
}

func (e *kafkaLogsProducer) Close(context.Context) error {
	return e.producer.Close()
}

func newMetricsExporter(config Config, set exporter.CreateSettings, marshalers map[string]MetricsMarshaler) (*kafkaMetricsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	producer, err := KafkaProducerFactory(config, set.Logger)
	if err != nil {
		return nil, err
	}

	return &kafkaMetricsProducer{
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil

}

// newTracesExporter creates Kafka exporter.
func newTracesExporter(config Config, set exporter.CreateSettings, marshalers map[string]TracesMarshaler) (*kafkaTracesProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	producer, err := KafkaProducerFactory(config, set.Logger)
	if err != nil {
		return nil, err
	}
	return &kafkaTracesProducer{
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}

func newLogsExporter(config Config, set exporter.CreateSettings, marshalers map[string]LogsMarshaler) (*kafkaLogsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	producer, err := KafkaProducerFactory(config, set.Logger)
	if err != nil {
		return nil, err
	}

	return &kafkaLogsProducer{
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil

}
