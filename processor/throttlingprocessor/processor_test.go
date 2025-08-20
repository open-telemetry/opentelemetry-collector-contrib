// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package throttlingprocessor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor/internal/metadata"
)

func Test_newProcessor(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expected    *throttlingProcessor
		expectedErr error
	}{
		{
			desc: "valid config",
			cfg: &Config{
				Interval:      5 * time.Second,
				Threshold:     100,
				KeyExpression: `Concat([attributes["key1"], attributes["key2"]], "_")`,
				Conditions:    []string{},
			},
			expected: &throttlingProcessor{
				interval: 5 * time.Second,
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logsSink := &consumertest.LogsSink{}
			settings := processortest.NewNopSettings(metadata.Type)

			if tc.expected != nil {
				tc.expected.nextConsumer = logsSink
			}

			actual, err := newProcessor(tc.cfg, logsSink, settings)
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
				require.Nil(t, actual)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected.interval, actual.interval)
				require.Equal(t, tc.expected.nextConsumer, actual.nextConsumer)
			}
		})
	}
}

func TestProcessorShutdownCtxError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)
	cfg := &Config{
		Interval:      5 * time.Second,
		Threshold:     1,
		KeyExpression: `Concat([attributes["key1"], attributes["key2"]], "_")`,
		Conditions:    []string{},
	}

	// Create a processor
	p, err := createLogsProcessor(t.Context(), settings, cfg, logsSink)
	require.NoError(t, err)

	// Start then stop the processor checking for errors
	err = p.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	err = p.Shutdown(ctx)
	require.NoError(t, err)
}

func TestProcessorCapabilities(t *testing.T) {
	p := &throttlingProcessor{}
	require.Equal(t, consumer.Capabilities{MutatesData: true}, p.Capabilities())
}

func TestShutdownBeforeStart(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)
	cfg := &Config{
		Interval:      5 * time.Second,
		Threshold:     1,
		KeyExpression: `Concat([attributes["key1"], attributes["key2"]], "_")`,
		Conditions:    []string{},
	}

	// Create a processor
	p, err := createLogsProcessor(t.Context(), settings, cfg, logsSink)
	require.NoError(t, err)
	require.NotPanics(t, func() {
		err := p.Shutdown(t.Context())
		require.NoError(t, err)
	})
}

func TestProcessorConsume(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)
	cfg := &Config{
		Interval:      5 * time.Second,
		Threshold:     1,
		KeyExpression: `Concat([attributes["key1"], attributes["key2"]], "_")`,
		Conditions:    []string{},
	}

	// Create a processor
	p, err := createLogsProcessor(t.Context(), settings, cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "basicLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 0
	}, 3*time.Second, 200*time.Millisecond)

	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "basicLogs.yaml"))
	require.NoError(t, err)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 1)

	require.NoError(t, plogtest.CompareLogs(expectedLogs, allSinkLogs[0], plogtest.IgnoreObservedTimestamp(), plogtest.IgnoreTimestamp(), plogtest.IgnoreLogRecordAttributeValue("first_observed_timestamp"), plogtest.IgnoreLogRecordAttributeValue("last_observed_timestamp")))

	// Cleanup
	err = p.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestProcessorConsumeCondition(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		Interval:      5 * time.Second,
		Threshold:     1,
		KeyExpression: `Concat([attributes["service.name"], attributes["env"]], "_")`,
		Conditions:    []string{`attributes["service.name"] == "service1"`},
	}

	// Create a processor
	p, err := createLogsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "conditionLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 1
	}, 3*time.Second, 200*time.Millisecond)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 1)

	expectedConsumedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "conditionConsumedLogs.yaml"))
	require.NoError(t, err)
	consumedLogs := allSinkLogs[0]

	require.NoError(t, plogtest.CompareLogs(expectedConsumedLogs, consumedLogs, plogtest.IgnoreLogRecordsOrder()))

	// Cleanup
	err = p.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestProcessorConsumeMultiConditions(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		Interval:      5 * time.Second,
		Threshold:     1,
		KeyExpression: `Concat([attributes["service.name"], attributes["env"]], "_")`,
		Conditions:    []string{`attributes["service.name"] == "service1"`, `attributes["service.name"] == "service2"`},
	}

	// Create a processor
	p, err := createLogsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "conditionLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 1
	}, 3*time.Second, 200*time.Millisecond)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 1)

	expectedConsumedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "multiConditionConsumedLogs.yaml"))
	require.NoError(t, err)
	consumedLogs := allSinkLogs[0]

	require.NoError(t, plogtest.CompareLogs(expectedConsumedLogs, consumedLogs, plogtest.IgnoreLogRecordsOrder()))

	// Cleanup
	err = p.Shutdown(t.Context())
	require.NoError(t, err)
}
