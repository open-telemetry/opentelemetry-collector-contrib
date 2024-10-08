// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
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
				Conditions:        []string{},
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
				Conditions:        []string{},
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
		Conditions:        []string{},
	}

	// Create a processor
	p, err := createLogsProcessor(context.Background(), settings, cfg, logsSink)
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
		Conditions:        []string{},
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := createLogsProcessor(context.Background(), settings, cfg, logsSink)
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
		Conditions:        []string{},
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := createLogsProcessor(context.Background(), settings, cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "basicLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(context.Background(), logs)
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
	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func Test_unsetLogsAreExportedOnShutdown(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
		Conditions:        []string{},
	}

	// Create & start a processor
	p, err := createLogsProcessor(context.Background(), processortest.NewNopSettings(), cfg, logsSink)
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

func TestProcessorConsumeCondition(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
		Conditions:        []string{`(attributes["ID"] == 1)`},
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := createLogsProcessor(context.Background(), processortest.NewNopSettings(), cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "conditionLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 4
	}, 3*time.Second, 200*time.Millisecond)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 2)

	expectedConsumedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "conditionConsumedLogs.yaml"))
	require.NoError(t, err)
	expectedDedupedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "conditionDedupedLogs.yaml"))
	require.NoError(t, err)

	consumedLogs := allSinkLogs[0]
	dedupedLogs := allSinkLogs[1]

	require.NoError(t, plogtest.CompareLogs(expectedConsumedLogs, consumedLogs, plogtest.IgnoreObservedTimestamp(), plogtest.IgnoreTimestamp(), plogtest.IgnoreLogRecordAttributeValue("first_observed_timestamp"), plogtest.IgnoreLogRecordAttributeValue("last_observed_timestamp")))
	require.NoError(t, plogtest.CompareLogs(expectedDedupedLogs, dedupedLogs, plogtest.IgnoreObservedTimestamp(), plogtest.IgnoreTimestamp(), plogtest.IgnoreLogRecordAttributeValue("first_observed_timestamp"), plogtest.IgnoreLogRecordAttributeValue("last_observed_timestamp")))

	// Cleanup
	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestProcessorConsumeMultipleConditions(t *testing.T) {
	logsSink := &consumertest.LogsSink{}
	cfg := &Config{
		LogCountAttribute: defaultLogCountAttribute,
		Interval:          1 * time.Second,
		Timezone:          defaultTimezone,
		Conditions:        []string{`attributes["ID"] == 1`, `attributes["ID"] == 3`},
		ExcludeFields: []string{
			fmt.Sprintf("%s.remove_me", attributeField),
		},
	}

	// Create a processor
	p, err := createLogsProcessor(context.Background(), processortest.NewNopSettings(), cfg, logsSink)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	logs, err := golden.ReadLogs(filepath.Join("testdata", "input", "conditionLogs.yaml"))
	require.NoError(t, err)

	// Consume the payload
	err = p.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Wait for the logs to be emitted
	require.Eventually(t, func() bool {
		return logsSink.LogRecordCount() > 3
	}, 3*time.Second, 200*time.Millisecond)

	allSinkLogs := logsSink.AllLogs()
	require.Len(t, allSinkLogs, 2)

	consumedLogs := allSinkLogs[0]
	dedupedLogs := allSinkLogs[1]

	expectedConsumedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "multipleConditionsConsumedLogs.yaml"))
	require.NoError(t, err)
	expectedDedupedLogs, err := golden.ReadLogs(filepath.Join("testdata", "expected", "multipleConditionsDedupedLogs.yaml"))
	require.NoError(t, err)

	err = plogtest.CompareLogs(expectedConsumedLogs, consumedLogs, plogtest.IgnoreObservedTimestamp(), plogtest.IgnoreTimestamp(), plogtest.IgnoreLogRecordAttributeValue("first_observed_timestamp"), plogtest.IgnoreLogRecordAttributeValue("last_observed_timestamp"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedDedupedLogs, dedupedLogs, plogtest.IgnoreObservedTimestamp(), plogtest.IgnoreTimestamp(), plogtest.IgnoreLogRecordAttributeValue("first_observed_timestamp"), plogtest.IgnoreLogRecordAttributeValue("last_observed_timestamp")))

	// Cleanup
	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}
