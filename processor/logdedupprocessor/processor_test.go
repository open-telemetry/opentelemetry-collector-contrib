// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
)

func Test_newProcessor(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expected    *logDedupProcessor
		expectedErr error
	}{
		{
			desc: "Timezone error",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          "bad timezone",
			},
			expected:    nil,
			expectedErr: errors.New("invalid timezone"),
		},
		{
			desc: "valid config",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
			},
			expected: &logDedupProcessor{
				emitInterval: defaultInterval,
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			logsSink := &consumertest.LogsSink{}
			settings := processortest.NewNopSettings()

			if tc.expected != nil {
				tc.expected.nextConsumer = logsSink
			}

			actual, err := newProcessor(tc.cfg, logsSink, settings)
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
				require.Nil(t, actual)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected.emitInterval, actual.emitInterval)
				require.NotNil(t, actual.aggregator)
				require.NotNil(t, actual.remover)
				require.Equal(t, tc.expected.nextConsumer, actual.nextConsumer)
			}
		})
	}
}

func TestProcessorShutdownCtxError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings()
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
	}

	// Create a processor
	p, err := newProcessor(cfg, logsSink, settings)
	require.NoError(t, err)

	// Start then stop the processor checking for errors
	err = p.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	err = p.Shutdown(ctx)
	require.NoError(t, err)
}

func TestProcessorCapabilities(t *testing.T) {
	p := &logDedupProcessor{}
	require.Equal(t, consumer.Capabilities{MutatesData: true}, p.Capabilities())
}

func TestShutdownBeforeStart(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings()
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := newProcessor(cfg, logsSink, settings)
	require.NoError(t, err)
	require.NotPanics(t, func() {
		err := p.Shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestProcessorConsume(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings()
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := newProcessor(cfg, logsSink, settings)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Create plog payload
	logRecord1 := generateTestLogRecord(t, "Body of the log")
	logRecord2 := generateTestLogRecord(t, "Body of the log")

	// Differ by timestamp and attribute to be removed
	logRecord1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Minute)))
	logRecord2.Attributes().PutBool("remove_me", false)

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutInt("one", 1)

	sl := rl.ScopeLogs().AppendEmpty()
	logRecord1.CopyTo(sl.LogRecords().AppendEmpty())
	logRecord2.CopyTo(sl.LogRecords().AppendEmpty())

	// Consume the payload
	err = p.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 0
	}, 3*time.Second, 200*time.Millisecond)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 1)

	consumedLogs := allSinkLogs[0]
	require.Equal(t, 1, consumedLogs.LogRecordCount())

	require.Equal(t, 1, consumedLogs.ResourceLogs().Len())
	consumedRl := consumedLogs.ResourceLogs().At(0)
	require.Equal(t, 1, consumedRl.ScopeLogs().Len())
	consumedSl := consumedRl.ScopeLogs().At(0)
	require.Equal(t, 1, consumedSl.LogRecords().Len())
	consumedLogRecord := consumedSl.LogRecords().At(0)

	countVal, ok := consumedLogRecord.Attributes().Get(cfg.LogCountAttribute)
	require.True(t, ok)
	require.Equal(t, int64(2), countVal.Int())

	// Cleanup
	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func Test_unsetLogsAreExportedOnShutdown(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
	}

	// Create & start a processor
	p, err := newProcessor(cfg, logsSink, processortest.NewNopSettings())
	require.NoError(t, err)
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Create logs payload
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()

	// Consume the logs
	err = p.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Shutdown the processor before it exports the logs
	err = p.Shutdown(context.Background())
	require.NoError(t, err)

	// Ensure the logs are exported
	exportedLogs := logsSink.AllLogs()
	require.Len(t, exportedLogs, 1)
}
