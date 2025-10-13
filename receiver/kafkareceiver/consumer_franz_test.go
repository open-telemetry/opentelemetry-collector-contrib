// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func setFranzGo(tb testing.TB, value bool) {
	currentFranzState := franzGoConsumerFeatureGate.IsEnabled()
	require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), value))
	tb.Cleanup(func() {
		require.NoError(tb, featuregate.GlobalRegistry().Set(franzGoConsumerFeatureGate.ID(), currentFranzState))
	})
}

func TestConsumerShutdownConsuming(t *testing.T) {
	type tCfg struct {
		mark        MessageMarking
		backOff     configretry.BackOffConfig
		returnError bool
	}
	type assertions struct {
		firstBatchProcessedCount  int64
		secondBatchProcessedCount int64
		committedOffset           int64
	}
	type testCase struct {
		name       string
		testConfig tCfg
		want       assertions
	}
	testCases := []testCase{
		{
			name:       "BackOff default marking",
			testConfig: tCfg{MessageMarking{}, configretry.NewDefaultBackOffConfig(), false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff default marking",
			testConfig: tCfg{MessageMarking{}, configretry.BackOffConfig{Enabled: false}, false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "BackOff default marking with error",
			testConfig: tCfg{MessageMarking{}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           2,
			},
		},
		{
			name:       "NoBackoff default marking with error",
			testConfig: tCfg{MessageMarking{}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "BackOff after marking",
			testConfig: tCfg{MessageMarking{After: true}, configretry.NewDefaultBackOffConfig(), false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff after marking",
			testConfig: tCfg{MessageMarking{After: true}, configretry.BackOffConfig{Enabled: false}, false},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		// With error
		{
			name:       "BackOff after marking with error",
			testConfig: tCfg{MessageMarking{After: true}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           0,
			},
		},
		{
			name:       "NoBackoff after marking with error",
			testConfig: tCfg{MessageMarking{After: true}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  1,
				secondBatchProcessedCount: 2,
				committedOffset:           0,
			},
		},
		// WithError OnError=true
		{
			name:       "BackOff after marking with error and OnError=true",
			testConfig: tCfg{MessageMarking{After: true, OnError: true}, configretry.NewDefaultBackOffConfig(), true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
		{
			name:       "NoBackoff after marking with error and OnError=true",
			testConfig: tCfg{MessageMarking{After: true, OnError: true}, configretry.BackOffConfig{Enabled: false}, true},
			want: assertions{
				firstBatchProcessedCount:  2,
				secondBatchProcessedCount: 4,
				committedOffset:           4,
			},
		},
	}

	// Create some traces for sending to the otlp_spans topic.
	const topic = "otlp_spans"
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	rs := []*kgo.Record{
		{Topic: topic, Value: data},
		{Topic: topic, Value: data},
	}

	testShutdown := func(tb testing.TB, testConfig tCfg, want assertions) {
		// Test that the consumer shuts down while consuming a message and
		// commits the offset after it's left the group.
		setFranzGo(tb, true)

		kafkaClient, cfg := mustNewFakeCluster(tb, kfake.SeedTopics(1, topic))
		cfg.ConsumerConfig = configkafka.ConsumerConfig{
			GroupID:    tb.Name(),
			AutoCommit: configkafka.AutoCommitConfig{Enable: true, Interval: 10 * time.Second},

			// Set MinFetchSize to ensure all records are fetched at once
			MinFetchSize: int32(len(data) * len(rs)),
		}
		cfg.ErrorBackOff = testConfig.backOff
		cfg.MessageMarking = testConfig.mark

		var called atomic.Int64
		var wg sync.WaitGroup
		settings, _, _ := mustNewSettings(tb)
		newConsumeFunc := func() (newConsumeMessageFunc, chan<- struct{}) {
			consuming := make(chan struct{})
			return func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
				return func(ctx context.Context, _ kafkaMessage, _ attribute.Set) error {
					wg.Add(1)
					defer wg.Done()

					<-consuming
					called.Add(1)
					// Wait for the consumer to shutdown.
					<-ctx.Done()
					if testConfig.returnError {
						return errors.New("error")
					}
					return nil
				}, nil
			}, consuming
		}

		test := func(tb testing.TB, expected int64) {
			ctx := t.Context()
			consumeFn, consuming := newConsumeFunc()
			consumer, e := newFranzKafkaConsumer(cfg, settings, []string{topic}, consumeFn)
			require.NoError(tb, e)
			require.NoError(tb, consumer.Start(ctx, componenttest.NewNopHost()))
			require.NoError(tb, kafkaClient.ProduceSync(ctx, rs...).FirstErr())

			select {
			case consuming <- struct{}{}:
				close(consuming) // Close the channel so the rest exit.
			case <-time.After(time.Second):
				tb.Fatal("expected to consume a message")
			}

			require.NoError(tb, consumer.Shutdown(ctx))
			wg.Wait() // Wait for the consume functions to exit.
			// Ensure that the consume function was called twice.
			assert.Equal(tb, expected, called.Load(), "consume function processed calls mismatch")
		}

		test(tb, want.firstBatchProcessedCount)
		test(tb, want.secondBatchProcessedCount)

		offsets, err := kadm.NewClient(kafkaClient).FetchOffsets(t.Context(), tb.Name())
		require.NoError(tb, err)
		// Lookup the last committed offset for partition 0
		offset, _ := offsets.Lookup(topic, 0)
		assert.Equal(tb, want.committedOffset, offset.At)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testShutdown(t, tc.testConfig, tc.want)
		})
	}
}

func TestConsumerShutdownNotStarted(t *testing.T) {
	setFranzGo(t, true)

	_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
	settings, _, _ := mustNewSettings(t)
	c, err := newFranzKafkaConsumer(cfg, settings, []string{"test"}, nil)
	require.NoError(t, err)

	for range 2 {
		require.NoError(t, c.Shutdown(t.Context()))
	}

	// Verify internal signal that there's nothing to shut down.
	// (Same package, so we can call the unexported helper.)
	require.False(t, c.triggerShutdown(), "triggerShutdown should indicate no-op when never started")
}

// TestRaceLostVsConsume verifies no data race occurs between concurrent
// message processing (which calls pc.add / pc.done) and partition revocation
// handling (lost() → pc.wait). It spins up a kfake cluster, floods them with
// records, and repeatedly invokes lost() while consumption is in-flight.
func TestRaceLostVsConsume(t *testing.T) {
	setFranzGo(t, true)
	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.ConsumerConfig = configkafka.ConsumerConfig{
		GroupID:      t.Name(),
		MaxFetchSize: 1, // Force a lot of iterations of consume()
		AutoCommit: configkafka.AutoCommitConfig{
			Enable: true, Interval: 100 * time.Millisecond,
		},
	}

	// Produce records.
	var rs []*kgo.Record
	for range 500 {
		traces := testdata.GenerateTraces(5)
		data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
		require.NoError(t, err)
		rs = append(rs, &kgo.Record{Topic: topic, Value: data})
	}
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), rs...).FirstErr())
	settings, _, _ := mustNewSettings(t)

	// Noop consume function.
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(context.Context, kafkaMessage, attribute.Set) error {
			return nil
		}, nil
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))

	done := make(chan struct{})
	// Hammer lost/assigned and rebalance in a goroutine.
	go func() {
		defer close(done)
		topicMap := map[string][]int32{topic: {0}}
		for range 2000 {
			c.lost(t.Context(), nil, topicMap, false)
			c.assigned(t.Context(), kafkaClient, topicMap)
			c.client.ForceRebalance()
			time.Sleep(time.Millisecond)
		}
	}()

	<-done
	require.NoError(t, c.Shutdown(t.Context()))
}

func TestLost(t *testing.T) {
	// It is possible that lost is called multiple times for the same partition
	// or called with a topic/partition that hasn't been assigned. This test
	// ensures that `lost` works without error in both cases.
	_, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, "test"))
	settings, _, _ := mustNewSettings(t)

	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ kafkaMessage, _ attribute.Set) error {
			return nil
		}, nil
	}
	c, err := newFranzKafkaConsumer(cfg, settings, []string{"test"}, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, c.Shutdown(t.Context())) }()

	// Call lost couple of times for same partition
	lostM := map[string][]int32{"test": {0}}
	c.lost(t.Context(), nil, lostM, false)
	c.lost(t.Context(), nil, lostM, false)

	// Call lost for a topic and partition that was not assigned
	c.lost(t.Context(), nil, map[string][]int32{"404": {0}}, true)
}

func TestFranzConsumer_UseLeaderEpoch_Smoke(t *testing.T) {
	setFranzGo(t, true)

	topic := "otlp_spans"
	kafkaClient, cfg := mustNewFakeCluster(t, kfake.SeedTopics(1, topic))
	cfg.UseLeaderEpoch = false // <-- exercise the option
	cfg.ConsumerConfig = configkafka.ConsumerConfig{
		GroupID:    t.Name(),
		AutoCommit: configkafka.AutoCommitConfig{Enable: true, Interval: 100 * time.Millisecond},
	}

	var called atomic.Int64
	settings, _, _ := mustNewSettings(t)
	consumeFn := func(component.Host, *receiverhelper.ObsReport, *metadata.TelemetryBuilder) (consumeMessageFunc, error) {
		return func(_ context.Context, _ kafkaMessage, _ attribute.Set) error {
			called.Add(1)
			return nil
		}, nil
	}

	// produce a couple of records
	traces := testdata.GenerateTraces(5)
	data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	rs := []*kgo.Record{
		{Topic: topic, Value: data},
		{Topic: topic, Value: data},
	}

	c, err := newFranzKafkaConsumer(cfg, settings, []string{topic}, consumeFn)
	require.NoError(t, err)
	require.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, kafkaClient.ProduceSync(t.Context(), rs...).FirstErr())

	// wait briefly for consumption
	deadline := time.After(2 * time.Second)
	for called.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected to consume 2 records, got %d", called.Load())
		case <-time.After(25 * time.Millisecond):
		}
	}

	require.NoError(t, c.Shutdown(t.Context()))
}

func TestMakeUseLeaderEpochAdjuster_ClearsEpoch(t *testing.T) {
	adj := makeClearLeaderEpochAdjuster()

	input := map[string]map[int32]kgo.Offset{
		"t": {
			0: kgo.NewOffset().At(42).WithEpoch(7),
			1: kgo.NewOffset().At(100), // no epoch set
		},
	}
	out, err := adj(t.Context(), input)
	require.NoError(t, err)

	require.Equal(t, kgo.NewOffset().At(42).WithEpoch(-1), out["t"][0])
	require.Equal(t, kgo.NewOffset().At(100).WithEpoch(-1), out["t"][1])
}
