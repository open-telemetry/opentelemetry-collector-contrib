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

// TestProcessPartitionBatchStopsWhenCancelled checks that the batch loop stops
// once the partition consumer is cancelled, so the rest of the batch stays
// unmarked. A revocation hands it to the next owner, a shutdown to the next run.
func TestProcessPartitionBatchStopsWhenCancelled(t *testing.T) {
	const topic = "test"
	const records = 10

	cases := []struct {
		name             string
		independent      bool
		marking          MessageMarking
		shutdown         bool
		cancelBeforeLoop bool
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
			// on_error marks records the pipeline refused, but a cancelled
			// record was never offered to it, so the interrupted one stays
			// unmarked too.
			name:             "legacy after marking with on_error",
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: -1,
		},
		{
			name:             "independent after marking with on_error",
			independent:      true,
			marking:          MessageMarking{After: true, OnError: true},
			wantConsumed:     1,
			wantMarkedOffset: -1,
		},
		{
			// Cancelled before the first record: nothing is consumed and
			// nothing is marked, so the whole batch is redelivered.
			name:             "cancelled before the first record",
			cancelBeforeLoop: true,
			wantConsumed:     0,
			wantMarkedOffset: -1,
		},
		{
			// Shutdown stops the loop too. Finishing the batch would mark
			// records against a context that is already cancelled, and the
			// next run would never see them again.
			name:             "shutdown stops the batch",
			shutdown:         true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
		},
		{
			name:             "independent shutdown stops the batch",
			independent:      true,
			shutdown:         true,
			wantConsumed:     1,
			wantMarkedOffset: 1,
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

			// Stands in for lost(), which cancels the partition context before
			// it waits for the in-flight call.
			cancelPartition := func() {
				partitionConsumer.cancelContext(errors.New("stopping processing"))
			}
			if tc.cancelBeforeLoop {
				cancelPartition()
			}

			consumed := 0
			consumer.consumeMessage = func(msgCtx context.Context, _ *kgo.Record, _ attribute.Set) error {
				consumed++
				if consumed == 1 && !tc.cancelBeforeLoop {
					cancelPartition()
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
				// Nothing counted as processed, so nothing may be marked and
				// there is no lag to report.
				require.Empty(t, marked, "an unprocessed batch must stay unmarked")
				return
			}
			require.Len(t, marked, 1)
			require.Equal(t, tc.wantMarkedOffset, marked[0].Offset,
				"marked records are committed and never redelivered")

			// The loop breaks instead of returning, so the records it did
			// process still report their lag.
			metadatatest.AssertEqualKafkaReceiverOffsetLag(t, tel, []metricdata.DataPoint[int64]{{
				Value: (records - 1) - (tc.wantMarkedOffset - 1),
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
