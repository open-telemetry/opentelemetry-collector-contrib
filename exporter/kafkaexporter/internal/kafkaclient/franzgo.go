// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

// MessageTooLargeError wraps a MessageTooLarge Kafka error with the actual
// record size that caused the rejection. The size is computed the same way as
// franz-go's Record.userSize: len(Key) + len(Value) + Σ(len(header.Key) + len(header.Value)).
type MessageTooLargeError struct {
	// RecordBytes is the user-visible size of the record (key + value + headers).
	RecordBytes int
	// MaxMessageBytes is the configured producer max message size.
	MaxMessageBytes int
	Err             error
}

func (e *MessageTooLargeError) Error() string {
	return fmt.Sprintf("record size %d exceeds max %d: %s", e.RecordBytes, e.MaxMessageBytes, e.Err.Error())
}
func (e *MessageTooLargeError) Unwrap() error { return e.Err }

// recordUserSize returns the user-visible size of a kgo.Record, matching
// franz-go's internal userSize calculation.
func recordUserSize(r *kgo.Record) int {
	s := len(r.Key) + len(r.Value)
	for _, h := range r.Headers {
		s += len(h.Key) + len(h.Value)
	}
	return s
}

// FranzSyncProducer is a wrapper around the franz-go client that implements
// the Producer interface. Allowing us to use the franz-go client while
// maintaining compatibility with the existing Kafka exporter code.
type FranzSyncProducer struct {
	client          *kgo.Client
	metadataKeys    []string
	recordHeaders   configopaque.MapList
	maxMessageBytes int
}

// NewFranzSyncProducer Franz-go producer from a kgo.Client and a Messenger.
func NewFranzSyncProducer(client *kgo.Client,
	metadataKeys []string,
	recordHeaders configopaque.MapList,
	maxMessageBytes int,
) *FranzSyncProducer {
	return &FranzSyncProducer{
		client:          client,
		metadataKeys:    metadataKeys,
		recordHeaders:   recordHeaders,
		maxMessageBytes: maxMessageBytes,
	}
}

// ExportData sends a batch of messages to Kafka
func (p *FranzSyncProducer) ExportData(ctx context.Context, msgs Messages) error {
	messages := makeFranzMessages(msgs, p.recordHeaders)
	setMessageHeaders(ctx, messages, p.metadataKeys)
	result := p.client.ProduceSync(ctx, messages...)
	var errs []error
	for _, r := range result {
		if r.Err == nil {
			continue
		}
		var err error
		if errors.Is(r.Err, kerr.MessageTooLarge) {
			err = fmt.Errorf("error exporting to topic %q: %w", r.Record.Topic,
				&MessageTooLargeError{RecordBytes: recordUserSize(r.Record), MaxMessageBytes: p.maxMessageBytes, Err: r.Err})
		} else {
			err = fmt.Errorf("error exporting to topic %q: %w", r.Record.Topic, r.Err)
		}
		// check if its defined as a non-retriable error by franzgo
		kgoErr := &kerr.Error{}
		if errors.As(r.Err, &kgoErr) && !kgoErr.Retriable {
			err = consumererror.NewPermanent(err)
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Close shuts down the producer and flushes any remaining messages.
func (p *FranzSyncProducer) Close() error {
	p.client.Close()
	return nil
}

func makeFranzMessages(messages Messages, recordHeaders configopaque.MapList) []*kgo.Record {
	msgs := make([]*kgo.Record, 0, messages.Count)
	for _, msg := range messages.TopicMessages {
		for _, message := range msg.Messages {
			record := &kgo.Record{Topic: msg.Topic}
			if message.Key != nil {
				record.Key = message.Key
			}
			if message.Value != nil {
				record.Value = message.Value
			}
			for _, pair := range recordHeaders {
				record.Headers = append(record.Headers, kgo.RecordHeader{
					Key:   pair.Name,
					Value: []byte(string(pair.Value)),
				})
			}
			msgs = append(msgs, record)
		}
	}
	return msgs
}
