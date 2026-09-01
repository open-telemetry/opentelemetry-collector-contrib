// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
)

// TestClearPauseReasons proves fetch resumes only after the last pause reason is cleared.
func TestClearPauseReasons(t *testing.T) {
	cases := []struct {
		name          string
		current       partitionPauseReason
		clear         partitionPauseReason
		wantResume    bool
		wantRemaining partitionPauseReason
	}{
		{
			name:  "does not resume without matching reason",
			clear: partitionPauseBackpressure,
		},
		{
			name:          "resumes after final reason clears",
			current:       partitionPauseBackpressure,
			clear:         partitionPauseBackpressure,
			wantResume:    true,
			wantRemaining: 0,
		},
		{
			name:          "does not resume while another reason remains",
			current:       partitionPauseBackpressure | partitionPauseRewind,
			clear:         partitionPauseBackpressure,
			wantRemaining: partitionPauseRewind,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			partitionConsumer := &pc{}
			partitionConsumer.pauseReasons.Store(uint32(tc.current))

			require.Equal(t, tc.wantResume, partitionConsumer.clearPauseReasons(tc.clear))
			require.Equal(t, uint32(tc.wantRemaining), partitionConsumer.pauseReasons.Load())
		})
	}
}

func TestProcessPartitionBatchMarkOwnership(t *testing.T) {
	const topic = "test"
	cases := []struct {
		name        string
		independent bool
		after       bool
		owner       bool
		wantMarked  bool
	}{
		{
			name:       "legacy marks after assignment changes",
			wantMarked: true,
		},
		{
			name:       "legacy after marks after assignment changes",
			after:      true,
			wantMarked: true,
		},
		{
			name:        "independent skips after assignment changes",
			independent: true,
		},
		{
			name:        "independent after skips after assignment changes",
			independent: true,
			after:       true,
		},
		{
			name:        "independent marks while owner",
			independent: true,
			owner:       true,
			wantMarked:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kafkaClient, cfg := mustNewMarkedFakeCluster(t, kfake.SeedTopics(1, topic))
			cfg.PartitionProcessing.Independent = tc.independent
			cfg.MessageMarking.After = tc.after
			settings, _, _ := mustNewSettings(t)
			consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
			require.NoError(t, err)
			consumer.client = kafkaClient
			consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
				return nil
			}

			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(nil) })
			partitionConsumer := &pc{
				ctx:    ctx,
				cancel: cancel,
				attrs:  attribute.NewSet(),
			}
			if tc.owner {
				consumer.assignments[topicPartition{topic: topic, partition: 0}] = partitionConsumer
			}

			batch := mailboxBatch(10)
			batch.Topic = topic
			batch.Records[0].Topic = topic
			batch.Records[0].Partition = 0
			consumer.processPartitionBatch(t.Context(), partitionConsumer, batch)

			marked := kafkaClient.MarkedOffsets()[topic]
			if tc.wantMarked {
				require.NotEmpty(t, marked)
				return
			}
			require.Empty(t, marked)
		})
	}
}
