// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

func createNoopReceiver(nextConsumer consumer.Logs) (*receiver, error) {
	emitter := NewLogEmitter(zap.NewNop().Sugar())

	pipe, err := pipeline.Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewConfig(),
			},
		},
	}.Build(zap.NewNop().Sugar())
	if err != nil {
		return nil, err
	}

	receiverID := component.NewID("test")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             receiverID,
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	if err != nil {
		return nil, err
	}

	return &receiver{
		id:        component.NewID("testReceiver"),
		pipe:      pipe,
		emitter:   emitter,
		consumer:  nextConsumer,
		logger:    zap.NewNop(),
		converter: NewConverter(zap.NewNop()),
		obsrecv:   obsrecv,
	}, nil
}

// BenchmarkEmitterToConsumer serves as a benchmark for entries going from the emitter to consumer,
// which follows this path: emitter -> receiver -> converter -> receiver -> consumer
func BenchmarkEmitterToConsumer(b *testing.B) {
	const (
		entryCount = 1_000_000
		hostsCount = 4
	)

	var (
		entries = complexEntriesForNDifferentHosts(entryCount, hostsCount)
	)

	cl := &consumertest.LogsSink{}
	logsReceiver, err := createNoopReceiver(cl)
	require.NoError(b, err)

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cl.Reset()

		go func() {
			ctx := context.Background()
			for _, e := range entries {
				_ = logsReceiver.emitter.Process(ctx, e)
			}
		}()

		require.Eventually(b,
			func() bool {
				return cl.LogRecordCount() == entryCount
			},
			30*time.Second, 5*time.Millisecond, "Did not receive all logs (only received %d)", cl.LogRecordCount(),
		)
	}
}

func TestEmitterToConsumer(t *testing.T) {
	const (
		entryCount = 1_000
		hostsCount = 4
	)

	entries := complexEntriesForNDifferentHosts(entryCount, hostsCount)

	cl := &consumertest.LogsSink{}
	logsReceiver, err := createNoopReceiver(cl)
	require.NoError(t, err)

	err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	go func() {
		ctx := context.Background()
		for _, e := range entries {
			require.NoError(t, logsReceiver.emitter.Process(ctx, e))
		}
	}()

	require.Eventually(t,
		func() bool {
			return cl.LogRecordCount() == entryCount
		},
		5*time.Second, 5*time.Millisecond, "Did not receive all logs (only received %d)", cl.LogRecordCount(),
	)

	// Wait for a small bit of time in order to let any potential extra entries drain out of the pipeline
	<-time.After(500 * time.Millisecond)

	require.Equal(t, entryCount, cl.LogRecordCount())
}
