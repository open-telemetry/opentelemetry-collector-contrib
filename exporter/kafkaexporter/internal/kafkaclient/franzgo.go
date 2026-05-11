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

// RecordHeader includes key-value pairs to be added as headers to Kafka records.
type RecordHeader struct {
	Name  string              `mapstructure:"name"`
	Value configopaque.String `mapstructure:"value"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// FranzSyncProducer is a wrapper around the franz-go client that implements
// the Producer interface. Allowing us to use the franz-go client while
// maintaining compatibility with the existing Kafka exporter code.
type FranzSyncProducer struct {
	client          *kgo.Client
	clientCancel    context.CancelFunc
	metadataKeys    []string
	recordHeaders   []kgo.RecordHeader
	maxMessageBytes int
}

// NewFranzSyncProducer Franz-go producer from a kgo.Client and a Messenger.
// clientCancel must cancel the context passed to kgo.WithContext when the client was created;
// it is called by Close to unblock any in-flight ProduceSync calls.
func NewFranzSyncProducer(client *kgo.Client,
	metadataKeys []string,
	recordHeaders []RecordHeader,
	maxMessageBytes int,
	clientCancel context.CancelFunc,
) *FranzSyncProducer {
	headers := make([]kgo.RecordHeader, 0, len(recordHeaders))
	for _, pair := range recordHeaders {
		headers = append(headers, kgo.RecordHeader{
			Key:   pair.Name,
			Value: []byte(pair.Value),
		})
	}

	return &FranzSyncProducer{
		client:          client,
		clientCancel:    clientCancel,
		metadataKeys:    metadataKeys,
		recordHeaders:   headers,
		maxMessageBytes: maxMessageBytes,
	}
}

// ExportData sends a batch of records to Kafka. It attaches configured
// record headers and per-call metadata-derived headers to each record before
// producing.
func (p *FranzSyncProducer) ExportData(ctx context.Context, records []*kgo.Record) error {
	metadataHeaders := metadataToHeaders(ctx, p.metadataKeys)
	var headers []kgo.RecordHeader
	if n := len(p.recordHeaders) + len(metadataHeaders); n > 0 {
		headers = make([]kgo.RecordHeader, 0, n)
		headers = append(headers, p.recordHeaders...)
		headers = append(headers, metadataHeaders...)
	}
	for _, r := range records {
		r.Headers = headers
	}
	result := p.client.ProduceSync(ctx, records...)
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

// Close shuts down the producer, unblocking any in-flight ExportData call.
func (p *FranzSyncProducer) Close(ctx context.Context) error {
	if p.clientCancel != nil {
		p.clientCancel()
	}
	done := make(chan struct{})
	go func() {
		p.client.Close()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
