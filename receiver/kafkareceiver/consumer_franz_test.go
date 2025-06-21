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

	testShutdown := func(tb testing.TB, testConfig tCfg, want assertions) {
		// Test that the consumer shuts down while consuming a message and
		// commits the offset after it's left the group.
		setFranzGo(tb, true)

		topic := "otlp_spans"
		kafkaClient, cfg := mustNewFakeCluster(tb, kfake.SeedTopics(1, topic))
		cfg.ConsumerConfig = configkafka.ConsumerConfig{
			GroupID:    tb.Name(),
			AutoCommit: configkafka.AutoCommitConfig{Enable: true, Interval: 10 * time.Second},
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

		// Send some traces to the otlp_spans topic.
		traces := testdata.GenerateTraces(5)
		data, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
		require.NoError(t, err)
		rs := []*kgo.Record{
			{Topic: topic, Value: data},
			{Topic: topic, Value: data},
		}

		test := func(tb testing.TB, expected int64) {
			ctx := context.Background()
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

		offsets, err := kadm.NewClient(kafkaClient).FetchOffsets(context.Background(), tb.Name())
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

	for i := 0; i < 2; i++ {
		require.EqualError(t, c.Shutdown(context.Background()),
			"kafka consumer: consumer isn't running")
	}
}
