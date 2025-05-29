// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"iter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

// Messenger is an interface that abstracts the data transformation and
// partitioning of a pdata type (plog.Logs, pmetric.Metrics, ptrace.Traces)
// into a sequence of messages that can be sent to Kafka.
type Messenger[T any] interface {
	// PartitionData returns an iterator that yields key-value pairs
	// where the key is the partition key, and the value is the pdata
	// type (plog.Logs, etc.)
	PartitionData(T) iter.Seq2[[]byte, T]

	// MarshalData marshals a pdata type into one or more messages.
	MarshalData(T) ([]marshaler.Message, error)

	// GetTopic returns the topic name for the given context and data.
	GetTopic(context.Context, T) string
}

// partitionMessages handles partitioning, marshaling, and message creation for a batch export.
func partitionMessages[T any, M any](ctx context.Context, data T,
	messenger Messenger[T],
	makeMessages func([]marshaler.Message, string) []M,
) ([]M, error) {
	var messages []M
	for key, pdata := range messenger.PartitionData(data) {
		partitionedMsgs, err := messenger.MarshalData(pdata)
		if err != nil {
			return nil, err
		}
		for i := range partitionedMsgs {
			if key != nil {
				partitionedMsgs[i].Key = key
			}
		}
		msgs := makeMessages(partitionedMsgs, messenger.GetTopic(ctx, pdata))
		messages = append(messages, msgs...)
	}
	return messages, nil
}
