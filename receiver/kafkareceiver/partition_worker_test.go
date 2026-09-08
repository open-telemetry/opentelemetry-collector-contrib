// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadatatest"
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

// TestProcessPartitionBatchStopsOnRevoke checks that the batch loop stops once
// the partition is revoked, so the rest of the batch stays unmarked and the next
// owner gets it. Shutdown has no next owner, so it finishes the batch.
func TestProcessPartitionBatchStopsOnRevoke(t *testing.T) {
	const topic = "test"
	const records = 10

	cases := []struct {
		name             string
		independent      bool
		marking          MessageMarking
		shutdown         bool
		revokeBeforeLoop bool
		wantConsumed     int
		wantMarkedOffset int64
	}{
		{
			// The default marks a record before processing it, so nothing but
			// the loop itself keeps the rest of the batch from being marked.
			name:             "legacy default marking",
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			name:             "independent default marking",
			independent:      true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			// after+on_error swallows the cancellation error, so the loop is
			// again the only thing that can stop it.
			name:             "legacy after marking with on_error",
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			name:             "independent after marking with on_error",
			independent:      true,
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			// Revoked before the first record: nothing is consumed and nothing
			// is marked, so the next owner gets the whole batch.
			name:             "revoked before the first record",
			revokeBeforeLoop: true,
			wantConsumed:     0,
			wantMarkedOffset: -1,
		},
		{
			// No one takes the rest of the batch over while the receiver is
			// leaving, so finish it instead of dropping it.
			name:             "shutdown finishes the batch",
			shutdown:         true,
			wantConsumed:     records,
			wantMarkedOffset: records,
		},
		{
			name:             "independent shutdown finishes the batch",
			independent:      true,
			shutdown:         true,
			wantConsumed:     records,
			wantMarkedOffset: records,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kafkaClient, cfg := mustNewMarkedFakeCluster(t, kfake.SeedTopics(1, topic))
			cfg.PartitionProcessing.Independent = tc.independent
			cfg.MessageMarking = tc.marking
			settings, tel, _ := mustNewSettings(t)
			consumer, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, nil, nil)
			require.NoError(t, err)
			consumer.client = kafkaClient
			if tc.shutdown {
				close(consumer.closing)
			}

			ctx, cancel := context.WithCancelCause(t.Context())
			t.Cleanup(func() { cancel(nil) })
			partitionConsumer := &pc{
				logger: settings.Logger,
				ctx:    ctx,
				cancel: cancel,
				attrs:  attribute.NewSet(),
			}
			consumer.assignments[topicPartition{topic: topic, partition: 0}] = partitionConsumer

			// Stands in for lost(), which cancels the partition context on every
			// revocation before it waits for the in-flight call.
			revoke := func() {
				partitionConsumer.cancelContext(errors.New("partition revoked"))
			}
			if tc.revokeBeforeLoop {
				revoke()
			}

			consumed := 0
			consumer.consumeMessage = func(msgCtx context.Context, _ *kgo.Record, _ attribute.Set) error {
				consumed++
				if consumed == 1 && !tc.revokeBeforeLoop {
					revoke()
				}
				return msgCtx.Err()
			}

			batch := kgo.FetchTopicPartition{Topic: topic}
			for i := range records {
				batch.Records = append(batch.Records, &kgo.Record{
					Topic: topic, Partition: 0, Offset: int64(i),
				})
			}
			batch.HighWatermark = records

			consumer.processPartitionBatch(partitionConsumer.ctx, partitionConsumer, batch)

			require.Equal(t, tc.wantConsumed, consumed,
				"records sent to the pipeline")
			marked := kafkaClient.MarkedOffsets()[topic]
			if tc.wantMarkedOffset < 0 {
				require.Empty(t, marked, "an unprocessed batch must stay unmarked")
			} else {
				require.Len(t, marked, 1)
				require.Equal(t, tc.wantMarkedOffset, marked[0].Offset,
					"marked records are committed and never redelivered")
			}

			if tc.wantConsumed == 0 {
				return
			}
			// The loop breaks instead of returning, so the records it did
			// process still report their lag.
			metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{{
				Value: (records - 1) - int64(tc.wantConsumed-1),
			}}, metricdatatest.IgnoreTimestamp())
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
