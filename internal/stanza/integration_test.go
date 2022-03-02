// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stanza

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/transformer/noop"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

func createNoopReceiver(workerCount int, nextConsumer consumer.Logs) (*receiver, error) {
	emitter := NewLogEmitter(
		LogEmitterWithLogger(zap.NewNop().Sugar()),
	)

	pipe, err := pipeline.Config{
		Operators: []operator.Config{
			{
				Builder: noop.NewNoopOperatorConfig(""),
			},
		},
	}.Build(zap.NewNop().Sugar())
	if err != nil {
		return nil, err
	}

	opts := []ConverterOption{
		WithLogger(zap.NewNop()),
	}

	if workerCount > 0 {
		opts = append(opts, WithWorkerCount(workerCount))
	}

	converter := NewConverter(opts...)

	receiverID, _ := config.NewComponentIDFromString("test")
	return &receiver{
		id:        config.NewComponentID("testReceiver"),
		pipe:      pipe,
		emitter:   emitter,
		consumer:  nextConsumer,
		logger:    zap.NewNop(),
		converter: converter,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             receiverID,
			ReceiverCreateSettings: componenttest.NewNopReceiverCreateSettings(),
		}),
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
		workerCounts = []int{1, 2, 4, 6, 8}
		entries      = complexEntriesForNDifferentHosts(entryCount, hostsCount)
	)

	for _, wc := range workerCounts {
		b.Run(fmt.Sprintf("worker_count=%d", wc), func(b *testing.B) {
			consumer := &mockLogsConsumer{}
			logsReceiver, err := createNoopReceiver(wc, consumer)
			require.NoError(b, err)

			err = logsReceiver.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(b, err)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				consumer.ResetReceivedCount()

				go func() {
					ctx := context.Background()
					for _, e := range entries {
						_ = logsReceiver.emitter.Process(ctx, e)
					}
				}()

				require.Eventually(b,
					func() bool {
						return consumer.Received() == entryCount
					},
					30*time.Second, 5*time.Millisecond, "Did not receive all logs (only received %d)", consumer.Received(),
				)
			}
		})
	}
}

func TestEmitterToConsumer(t *testing.T) {
	const (
		entryCount  = 1_000
		hostsCount  = 4
		workerCount = 2
	)

	entries := complexEntriesForNDifferentHosts(entryCount, hostsCount)

	consumer := &mockLogsConsumer{}
	logsReceiver, err := createNoopReceiver(workerCount, consumer)
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
			return consumer.Received() == entryCount
		},
		5*time.Second, 5*time.Millisecond, "Did not receive all logs (only received %d)", consumer.Received(),
	)

	// Wait for a small bit of time in order to let any potential extra entries drain out of the pipeline
	<-time.After(500 * time.Millisecond)

	require.Equal(t, entryCount, consumer.Received())
}
