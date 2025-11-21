// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func TestFailoverModes(t *testing.T) {
	testCases := []struct {
		name         string
		failoverMode FailoverMode
	}{
		{
			name:         "standard_mode",
			failoverMode: FailoverModeStandard,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var sinkFirst, sinkSecond consumertest.TracesSink
			tracesFirst := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/first")
			tracesSecond := pipeline.NewIDWithName(pipeline.SignalTraces, "traces/second")

			cfg := &Config{
				PipelinePriority: [][]pipeline.ID{{tracesFirst}, {tracesSecond}},
				FailoverMode:     tc.failoverMode,
				RetryInterval:    50 * time.Millisecond,
			}

			router := connector.NewTracesRouter(map[pipeline.ID]consumer.Traces{
				tracesFirst:  &sinkFirst,
				tracesSecond: &sinkSecond,
			})

			conn, err := NewFactory().CreateTracesToTraces(t.Context(),
				connectortest.NewNopSettings(metadata.Type), cfg, router.(consumer.Traces))
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, conn.Shutdown(t.Context()))
			}()

			tr := sampleTrace()
			err = conn.ConsumeTraces(t.Context(), tr)
			assert.NoError(t, err)

			assert.Len(t, sinkFirst.AllTraces(), 1)
			assert.Empty(t, sinkSecond.AllTraces())

			sinkFirst.Reset()
			sinkSecond.Reset()

			failoverConnector := conn.(*tracesFailover)
			failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(assert.AnError))

			err = conn.ConsumeTraces(t.Context(), tr)
			assert.NoError(t, err)

			assert.Empty(t, sinkFirst.AllTraces())
			assert.Len(t, sinkSecond.AllTraces(), 1)
		})
	}
}
