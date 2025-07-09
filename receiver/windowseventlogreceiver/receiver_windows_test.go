// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := newFactoryAdapter()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := newFactoryAdapter()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.Equal(t, createTestConfig(), cfg)
}

func TestCreateWithInvalidInputConfig(t *testing.T) {
	cfg := &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{},
		InputConfig: func() windows.Config {
			c := windows.NewConfig()
			c.StartAt = "middle"
			return *c
		}(),
	}

	_, err := newFactoryAdapter().CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		new(consumertest.LogsSink),
	)
	require.Error(t, err, "receiver creation should fail if given invalid input config")
}

// BenchmarkReadWindowsEventLogger benchmarks the performance of reading Windows Event Log events.
// This benchmark is not good to measure time performance since it uses a "eventually" construct
// to wait for the logs to be read. However, it is good to evaluate memory usage.
func BenchmarkReadWindowsEventLogger(b *testing.B) {
	b.StopTimer()

	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "10",
			count: 10,
		},
		{
			name:  "100",
			count: 100,
		},
		{
			name:  "1_000",
			count: 1_000,
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Set up the receiver and sink.
				ctx := context.Background()
				factory := newFactoryAdapter()
				createSettings := receivertest.NewNopSettings(metadata.Type)
				cfg := createTestConfig()
				cfg.InputConfig.StartAt = "beginning"
				cfg.InputConfig.MaxReads = tt.count
				sink := new(consumertest.LogsSink)

				receiver, err := factory.CreateLogs(ctx, createSettings, cfg, sink)
				require.NoError(b, err)

				_ = receiver.Start(ctx, componenttest.NewNopHost())
				b.StartTimer()
				assert.Eventually(b, func() bool {
					return sink.LogRecordCount() >= tt.count
				}, 20*time.Second, 250*time.Millisecond)
				b.StopTimer()
				_ = receiver.Shutdown(ctx)
			}
		})
	}
}

func TestReadWindowsEventLogger(t *testing.T) {
	logMessage := "Test log"
	src := "otel-windowseventlogreceiver-test"
	uninstallEventSource, err := assertEventSourceInstallation(t, src)
	defer uninstallEventSource()
	require.NoError(t, err)

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopSettings(metadata.Type)
	cfg := createTestConfig()
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogs(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()
	// Start launches nested goroutines, give them a chance to run before logging the test event(s).
	time.Sleep(3 * time.Second)

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	records := assertExpectedLogRecords(t, sink, src, 1)
	require.Len(t, records, 1)
	record := records[0]
	body := record.Body().Map().AsRaw()

	require.Equal(t, logMessage, body["message"])

	eventData := body["event_data"]
	eventDataMap, ok := eventData.(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{
		"data": []any{map[string]any{"": "Test log"}},
	}, eventDataMap)

	eventID := body["event_id"]
	require.NotNil(t, eventID)

	eventIDMap, ok := eventID.(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(10), eventIDMap["id"])
}

func TestReadWindowsEventLoggerWithQuery(t *testing.T) {
	logMessage := "Test log"
	src := "otel-windowseventlogreceiver-test"
	uninstallEventSource, err := assertEventSourceInstallation(t, src)
	defer uninstallEventSource()
	require.NoError(t, err)

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopSettings(metadata.Type)
	cfg := createTestConfigWithQuery()
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogs(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()
	// Start launches nested goroutines, give them a chance to run before logging the test event(s).
	time.Sleep(3 * time.Second)

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	records := assertExpectedLogRecords(t, sink, src, 1)
	require.Len(t, records, 1)
	record := records[0]
	body := record.Body().Map().AsRaw()

	require.Equal(t, logMessage, body["message"])

	eventData := body["event_data"]
	eventDataMap, ok := eventData.(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{
		"data": []any{map[string]any{"": "Test log"}},
	}, eventDataMap)

	eventID := body["event_id"]
	require.NotNil(t, eventID)

	eventIDMap, ok := eventID.(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(10), eventIDMap["id"])
}

func TestReadWindowsEventLoggerRaw(t *testing.T) {
	logMessage := "Test log"
	src := "otel-windowseventlogreceiver-test"
	uninstallEventSource, err := assertEventSourceInstallation(t, src)
	defer uninstallEventSource()
	require.NoError(t, err)

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopSettings(metadata.Type)
	cfg := createTestConfig()
	cfg.InputConfig.Raw = true
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogs(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()
	// Start launches nested goroutines, give them a chance to run before logging the test event(s).
	time.Sleep(3 * time.Second)

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	records := assertExpectedLogRecords(t, sink, src, 1)
	require.Len(t, records, 1)
	record := records[0]
	body := record.Body().AsString()
	bodyStruct := struct {
		Data string `xml:"EventData>Data"`
	}{}
	err = xml.Unmarshal([]byte(body), &bodyStruct)
	require.NoError(t, err)

	require.Equal(t, logMessage, bodyStruct.Data)
}

func TestExcludeProvider(t *testing.T) {
	logMessage := "Test log"
	excludedSrc := "otel-excluded-windowseventlogreceiver-test"
	notExcludedSrc := "otel-windowseventlogreceiver-test"
	testSources := []string{excludedSrc, notExcludedSrc}

	for _, src := range testSources {
		uninstallEventSource, err := assertEventSourceInstallation(t, src)
		defer uninstallEventSource()
		require.NoError(t, err)
	}

	tests := []struct {
		name string
		raw  bool
	}{
		{
			name: "with_EventXML",
		},
		{
			name: "with_Raw",
			raw:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			factory := newFactoryAdapter()
			createSettings := receivertest.NewNopSettings(metadata.Type)
			cfg := createTestConfig()
			cfg.InputConfig.Raw = tt.raw
			cfg.InputConfig.ExcludeProviders = []string{excludedSrc}
			sink := new(consumertest.LogsSink)

			receiver, err := factory.CreateLogs(ctx, createSettings, cfg, sink)
			require.NoError(t, err)

			err = receiver.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, receiver.Shutdown(ctx))
			}()
			// Start launches nested goroutines, give them a chance to run before logging the test event(s).
			time.Sleep(3 * time.Second)

			for _, src := range testSources {
				logger, err := eventlog.Open(src)
				require.NoError(t, err)
				defer logger.Close()

				err = logger.Info(10, logMessage)
				require.NoError(t, err)
			}

			records := assertExpectedLogRecords(t, sink, notExcludedSrc, 1)
			assert.Len(t, records, 1)

			records = filterAllLogRecordsBySource(t, sink, excludedSrc)
			assert.Empty(t, records)
		})
	}
}

func createTestConfig() *WindowsLogConfig {
	return &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() windows.Config {
			c := windows.NewConfig()
			c.Channel = "application"
			c.StartAt = "end"
			return *c
		}(),
	}
}

func createTestConfigWithQuery() *WindowsLogConfig {
	queryXML := `
    <QueryList>
      <Query Id="0">
        <Select Path="Application">*</Select>
      </Query>
    </QueryList>
  `
	return &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() windows.Config {
			c := windows.NewConfig()
			c.Query = &queryXML
			c.StartAt = "end"
			return *c
		}(),
	}
}

// assertEventSourceInstallation installs an event source and verifies that the registry key was created.
// It returns a function that can be used to uninstall the event source, that function is never nil
func assertEventSourceInstallation(t *testing.T, src string) (uninstallEventSource func(), err error) {
	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	if err != nil && strings.HasSuffix(err.Error(), " registry key already exists") {
		// If the event source already exists ignore the error
		err = nil
	}
	uninstallEventSource = func() {
		assert.NoError(t, eventlog.Remove(src))
	}
	assert.NoError(t, err)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		rk, err := registry.OpenKey(registry.LOCAL_MACHINE, "SYSTEM\\CurrentControlSet\\Services\\EventLog\\Application\\"+src, registry.QUERY_VALUE)
		assert.NoError(c, err)
		defer rk.Close()
		_, _, err = rk.GetIntegerValue("TypesSupported")
		assert.NoError(c, err)
	}, 10*time.Second, 250*time.Millisecond)

	return
}

//nolint:unparam // expectedEventCount might be greater than one in the future
func assertExpectedLogRecords(t *testing.T, sink *consumertest.LogsSink, expectedEventSrc string, expectedEventCount int) []plog.LogRecord {
	var actualLogRecords []plog.LogRecord

	// We can't use assert.Eventually because the condition function is launched in a separate goroutine
	// and we want to return the slice filled by the condition function. Use a local condition check
	// to avoid data race conditions.
	startTime := time.Now()
	actualLogRecords = filterAllLogRecordsBySource(t, sink, expectedEventSrc)
	for len(actualLogRecords) != expectedEventCount && time.Since(startTime) < 10*time.Second {
		time.Sleep(250 * time.Millisecond)
		actualLogRecords = filterAllLogRecordsBySource(t, sink, expectedEventSrc)
	}

	require.Len(t, actualLogRecords, expectedEventCount)

	return actualLogRecords
}

func filterAllLogRecordsBySource(t *testing.T, sink *consumertest.LogsSink, src string) (filteredLogRecords []plog.LogRecord) {
	for _, logs := range sink.AllLogs() {
		resourceLogsSlice := logs.ResourceLogs()
		for i := 0; i < resourceLogsSlice.Len(); i++ {
			resourceLogs := resourceLogsSlice.At(i)
			scopeLogsSlice := resourceLogs.ScopeLogs()
			for j := 0; j < scopeLogsSlice.Len(); j++ {
				logRecords := scopeLogsSlice.At(j).LogRecords()
				for k := 0; k < logRecords.Len(); k++ {
					logRecord := logRecords.At(k)
					if extractEventSourceFromLogRecord(t, logRecord) == src {
						filteredLogRecords = append(filteredLogRecords, logRecord)
					}
				}
			}
		}
	}

	return
}

func extractEventSourceFromLogRecord(t *testing.T, logRecord plog.LogRecord) string {
	eventMap := logRecord.Body().Map()
	if !reflect.DeepEqual(eventMap, pcommon.Map{}) {
		eventProviderMap := eventMap.AsRaw()["provider"]
		if providerMap, ok := eventProviderMap.(map[string]any); ok {
			return providerMap["name"].(string)
		}
		require.Fail(t, "Failed to extract event source from log record")
	}

	// This is a raw event log record. Extract the event source from the XML body string.
	bodyString := logRecord.Body().AsString()
	var eventXML windows.EventXML
	require.NoError(t, xml.Unmarshal([]byte(bodyString), &eventXML))
	return eventXML.Provider.Name
}
