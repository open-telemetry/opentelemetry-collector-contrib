// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func TestBeforeMarkingCancellation(t *testing.T) {
	// A canceled mark-before batch commits the failed record without
	// requesting a rewind.
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionProcessing.Independent = true
	cfg.ConsumerConfig.AutoCommit.Enable = false
	cfg.ErrorBackOff = configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: time.Hour,
		MaxInterval:     time.Hour,
		MaxElapsedTime:  2 * time.Hour,
	}

	settings, _, _ := mustNewSettings(t)
	consumer, err := newFranzKafkaConsumer(cfg, settings, nil, nil, nil)
	require.NoError(t, err)
	consumer.consumeMessage = func(context.Context, *kgo.Record, attribute.Set) error {
		return errors.New("processing failed")
	}

	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(errors.New("partition revoked"))
	partitionConsumer := &pc{
		ctx:     ctx,
		logger:  zap.NewNop(),
		backOff: newExponentialBackOff(cfg.ErrorBackOff),
	}
	record := &kgo.Record{
		Topic:     "test",
		Partition: 0,
		Offset:    1,
	}

	result := consumer.processPartitionBatch(ctx, partitionConsumer, kgo.FetchTopicPartition{
		Topic: record.Topic,
		FetchPartition: kgo.FetchPartition{
			Partition: record.Partition,
			Records:   []*kgo.Record{record},
		},
	})

	require.Same(t, record, result.commitRecord)
	require.Nil(t, result.rewindRecord)
	require.True(t, result.terminal)
}

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
