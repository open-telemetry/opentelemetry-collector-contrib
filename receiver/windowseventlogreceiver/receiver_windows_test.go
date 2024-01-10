// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windowseventlogreceiver

import (
	"context"
	"encoding/xml"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	assert.Equal(t, createTestConfig(), cfg)
}

func TestCreateWithInvalidInputConfig(t *testing.T) {
	t.Parallel()

	cfg := &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{},
		InputConfig: func() windows.Config {
			c := windows.NewConfig()
			c.StartAt = "middle"
			return *c
		}(),
	}

	_, err := newFactoryAdapter().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		new(consumertest.LogsSink),
	)
	require.Error(t, err, "receiver creation should fail if given invalid input config")
}

func TestReadWindowsEventLogger(t *testing.T) {
	logMessage := "Test log"

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopCreateSettings()
	cfg := createTestConfig()
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogsReceiver(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	src := "otel"
	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, eventlog.Remove(src))
	}()

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	logsReceived := func() bool {
		return sink.LogRecordCount() == 1
	}

	// logs sometimes take a while to be written, so a substantial wait buffer is needed
	require.Eventually(t, logsReceived, 10*time.Second, 200*time.Millisecond)
	results := sink.AllLogs()
	require.Len(t, results, 1)

	records := results[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 1, records.Len())

	record := records.At(0)
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

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopCreateSettings()
	cfg := createTestConfig()
	cfg.InputConfig.Raw = true
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogsReceiver(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	src := "otel"
	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	defer func() {
		require.NoError(t, eventlog.Remove(src))
	}()
	require.NoError(t, err)

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	logsReceived := func() bool {
		return sink.LogRecordCount() == 1
	}

	// logs sometimes take a while to be written, so a substantial wait buffer is needed
	require.Eventually(t, logsReceived, 10*time.Second, 200*time.Millisecond)
	results := sink.AllLogs()
	require.Len(t, results, 1)

	records := results[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 1, records.Len())

	record := records.At(0)
	body := record.Body().AsString()
	bodyStruct := struct {
		Data string `xml:"EventData>Data"`
	}{}
	err = xml.Unmarshal([]byte(body), &bodyStruct)
	require.NoError(t, err)

	require.Equal(t, logMessage, bodyStruct.Data)
}

func TestReadWindowsEventLoggerWithExcludeProvider(t *testing.T) {
	logMessage := "Test log"
	src := "otel"

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopCreateSettings()
	cfg := createTestConfig()
	cfg.InputConfig.ExcludeProviders = []string{src}
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogsReceiver(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, eventlog.Remove(src))
	}()

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	logsReceived := func() bool {
		return sink.LogRecordCount() == 0
	}

	// logs sometimes take a while to be written, so a substantial wait buffer is needed
	require.Eventually(t, logsReceived, 10*time.Second, 200*time.Millisecond)
	results := sink.AllLogs()
	require.Len(t, results, 0)
}

func TestReadWindowsEventLoggerRawWithExcludeProvider(t *testing.T) {
	logMessage := "Test log"
	src := "otel"

	ctx := context.Background()
	factory := newFactoryAdapter()
	createSettings := receivertest.NewNopCreateSettings()
	cfg := createTestConfig()
	cfg.InputConfig.Raw = true
	cfg.InputConfig.ExcludeProviders = []string{src}
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogsReceiver(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, receiver.Shutdown(ctx))
	}()

	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	defer func() {
		require.NoError(t, eventlog.Remove(src))
	}()
	require.NoError(t, err)

	logger, err := eventlog.Open(src)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Info(10, logMessage)
	require.NoError(t, err)

	logsReceived := func() bool {
		return sink.LogRecordCount() == 0
	}

	// logs sometimes take a while to be written, so a substantial wait buffer is needed
	require.Eventually(t, logsReceived, 10*time.Second, 200*time.Millisecond)
	results := sink.AllLogs()
	require.Len(t, results, 0)
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
