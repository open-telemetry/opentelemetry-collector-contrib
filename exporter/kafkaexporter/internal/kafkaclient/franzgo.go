// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

// FranzSyncProducer is a wrapper around the franz-go client that implements
// the Producer interface. Allowing us to use the franz-go client while
// maintaining compatibility with the existing Kafka exporter code.
type FranzSyncProducer[T any] struct {
	client       *kgo.Client
	messenger    Messenger[T]
	metadataKeys []string
}

// NewFranzSyncProducer Franz-go producer from a kgo.Client and a Messenger.
func NewFranzSyncProducer[T any](
	client *kgo.Client,
	messenger Messenger[T],
	metadataKeys []string,
) FranzSyncProducer[T] {
	return FranzSyncProducer[T]{
		client:       client,
		messenger:    messenger,
		metadataKeys: metadataKeys,
	}
}

// ExportData sends a batch of messages to Kafka
func (p FranzSyncProducer[T]) ExportData(ctx context.Context, data T) error {
	messages, err := partitionMessages(ctx,
		data, p.messenger, makeFranzMessages,
	)
	if err != nil || len(messages) == 0 {
		return err
	}
	setMessageHeaders(ctx, messages, p.metadataKeys,
		func(key string, value []byte) kgo.RecordHeader {
			return kgo.RecordHeader{Key: key, Value: value}
		},
		func(m *kgo.Record) []kgo.RecordHeader { return m.Headers },
		func(m *kgo.Record, h []kgo.RecordHeader) { m.Headers = h },
	)
	result := p.client.ProduceSync(ctx, messages...)
	var errs []error
	for _, r := range result {
		if r.Err != nil {
			fmt.Println(r.Err.Error())
			errs = append(errs, r.Err)
		}
	}
	return errors.Join(errs...)
}

// Close shuts down the producer and flushes any remaining messages.
func (p FranzSyncProducer[T]) Close() error {
	p.client.Close()
	return nil
}

func makeFranzMessages(messages []marshaler.Message, topic string) []*kgo.Record {
	msgs := make([]*kgo.Record, len(messages))
	for i, message := range messages {
		msgs[i] = &kgo.Record{Topic: topic}
		if message.Key != nil {
			msgs[i].Key = message.Key
		}
		if message.Value != nil {
			msgs[i].Value = message.Value
		}
	}
	return msgs
}
