// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package windowseventlogreceiver

import (
	"context"
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
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
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

	_, err := NewFactory().CreateLogsReceiver(
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
	factory := NewFactory()
	createSettings := receivertest.NewNopCreateSettings()
	cfg := createTestConfig()
	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogsReceiver(ctx, createSettings, cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer receiver.Shutdown(ctx)

	src := "otel"
	err = eventlog.InstallAsEventCreate(src, eventlog.Info|eventlog.Warning|eventlog.Error)
	require.NoError(t, err)
	defer eventlog.Remove(src)

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
	eventDataMap, ok := eventData.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, map[string]interface{}{}, eventDataMap)

	eventID := body["event_id"]
	require.NotNil(t, eventID)

	eventIDMap, ok := eventID.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, int64(10), eventIDMap["id"])
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
