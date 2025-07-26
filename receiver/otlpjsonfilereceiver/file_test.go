// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFileProfilesReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.ProfilesSink)
	receiver, err := factory.(xreceiver.Factory).CreateProfiles(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	require.NoError(t, err)

	pd := testdata.GenerateProfiles(1)
	marshaler := &pprofile.JSONMarshaler{}
	b, err := marshaler.MarshalProfiles(pd)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "profiles.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllProfiles(), 1)

	// Verify offset attributes are NOT present by default (backward compatibility)
	profiles := sink.AllProfiles()[0]
	resourceProfile := profiles.ResourceProfiles().At(0)

	_, offsetExists := resourceProfile.Resource().Attributes().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := resourceProfile.Resource().Attributes().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileTracesReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.TracesSink)
	receiver, err := factory.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	require.NoError(t, err)

	td := testdata.GenerateTraces(1)
	marshaler := &ptrace.JSONMarshaler{}
	b, err := marshaler.MarshalTraces(td)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "traces.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllTraces(), 1)

	// Verify default behavior: only file name, no offset attributes
	traces := sink.AllTraces()[0]
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	// include_file_name is true by default
	fileNameVal, exists := span.Attributes().Get("log.file.name")
	assert.True(t, exists, "log.file.name should exist")
	assert.Equal(t, "traces.json", fileNameVal.Str())

	// Offset attributes should NOT be present by default (backward compatibility)
	_, offsetExists := span.Attributes().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := span.Attributes().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileMetricsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetrics(1)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllMetrics(), 1)

	// Verify default behavior: only file name, no offset attributes
	metrics := sink.AllMetrics()[0]
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	// include_file_name is true by default
	fileNameVal, exists := metric.Metadata().Get("log.file.name")
	assert.True(t, exists, "log.file.name should exist")
	assert.Equal(t, "metrics.json", fileNameVal.Str())

	// Offset attributes should NOT be present by default (backward compatibility)
	_, offsetExists := metric.Metadata().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := metric.Metadata().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileMetricsReceiverWithReplay(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	cfg.ReplayFile = true
	cfg.PollInterval = 5 * time.Second
	cfg.IncludeFileName = false

	sink := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetrics(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0o600)
	assert.NoError(t, err)

	// Wait for the first poll to complete.
	time.Sleep(cfg.PollInterval + time.Second)
	require.Len(t, sink.AllMetrics(), 1)

	// Verify no offset attributes by default (file name disabled)
	metrics := sink.AllMetrics()[0]
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	_, offsetExists := metric.Metadata().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := metric.Metadata().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	// Reset the sink and assert that the next poll replays all the existing metrics.
	sink.Reset()
	time.Sleep(cfg.PollInterval + time.Second)
	require.Len(t, sink.AllMetrics(), 1)

	// Verify no offset attributes after replay
	replayedMetrics := sink.AllMetrics()[0]
	replayedMetric := replayedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	_, offsetExists = replayedMetric.Metadata().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist after replay by default")

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileLogsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	ld := testdata.GenerateLogs(1)
	marshaler := &plog.JSONMarshaler{}
	b, err := marshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllLogs(), 1)

	// Verify default behavior: only file name, no offset attributes
	logs := sink.AllLogs()[0]
	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// include_file_name is true by default
	fileNameVal, exists := logRecord.Attributes().Get("log.file.name")
	assert.True(t, exists, "log.file.name should exist")
	assert.Equal(t, "logs.json", fileNameVal.Str())

	// Offset attributes should NOT be present by default (backward compatibility)
	_, offsetExists := logRecord.Attributes().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := logRecord.Attributes().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func testdataConfigYamlAsMap() *Config {
	return &Config{
		Config: fileconsumer.Config{
			Resolver: attrs.Resolver{
				IncludeFileName:         true,
				IncludeFilePath:         false,
				IncludeFileNameResolved: false,
				IncludeFilePathResolved: false,
			},
			PollInterval:       200 * time.Millisecond,
			Encoding:           "utf-8",
			StartAt:            "end",
			FingerprintSize:    1000,
			InitialBufferSize:  16 * 1024,
			MaxLogSize:         1024 * 1024,
			MaxConcurrentFiles: 1024,
			FlushPeriod:        500 * time.Millisecond,
			Criteria: matcher.Criteria{
				Include: []string{"/var/log/*.log"},
				Exclude: []string{"/var/log/example.log"},
			},
		},
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, testdataConfigYamlAsMap(), cfg)
}

func TestFileMixedSignals(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	cfg.IncludeFileName = false
	cs := receivertest.NewNopSettings(metadata.Type)
	ms := new(consumertest.MetricsSink)
	mr, err := factory.CreateMetrics(context.Background(), cs, cfg, ms)
	assert.NoError(t, err)
	err = mr.Start(context.Background(), nil)
	assert.NoError(t, err)
	ts := new(consumertest.TracesSink)
	tr, err := factory.CreateTraces(context.Background(), cs, cfg, ts)
	assert.NoError(t, err)
	err = tr.Start(context.Background(), nil)
	assert.NoError(t, err)
	ls := new(consumertest.LogsSink)
	lr, err := factory.CreateLogs(context.Background(), cs, cfg, ls)
	assert.NoError(t, err)
	err = lr.Start(context.Background(), nil)
	assert.NoError(t, err)
	ps := new(consumertest.ProfilesSink)
	pr, err := factory.(xreceiver.Factory).CreateProfiles(context.Background(), cs, cfg, ps)
	assert.NoError(t, err)
	err = pr.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetrics(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	td := testdata.GenerateTraces(2)
	tmarshaler := &ptrace.JSONMarshaler{}
	b2, err := tmarshaler.MarshalTraces(td)
	assert.NoError(t, err)
	ld := testdata.GenerateLogs(5)
	lmarshaler := &plog.JSONMarshaler{}
	b3, err := lmarshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	pd := testdata.GenerateProfiles(5)
	pmarshaler := &pprofile.JSONMarshaler{}
	b4, err := pmarshaler.MarshalProfiles(pd)
	assert.NoError(t, err)
	b = append(b, '\n')
	b = append(b, b2...)
	b = append(b, '\n')
	b = append(b, b3...)
	b = append(b, '\n')
	b = append(b, b4...)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, ms.AllMetrics(), 1)
	require.Len(t, ts.AllTraces(), 1)
	require.Len(t, ls.AllLogs(), 1)
	require.Len(t, ps.AllProfiles(), 1)

	// Verify no offset attributes by default (backward compatibility)
	metrics := ms.AllMetrics()[0]
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

	_, offsetExists := metric.Metadata().Get("log.file.record_offset")
	assert.False(t, offsetExists, "log.file.record_offset should not exist by default")

	_, recordNumExists := metric.Metadata().Get("log.file.record_number")
	assert.False(t, recordNumExists, "log.file.record_number should not exist by default")

	err = mr.Shutdown(context.Background())
	assert.NoError(t, err)
	err = tr.Shutdown(context.Background())
	assert.NoError(t, err)
	err = lr.Shutdown(context.Background())
	assert.NoError(t, err)
	err = pr.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestEmptyLine(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	cs := receivertest.NewNopSettings(metadata.Type)
	t.Run("metrics receiver", func(t *testing.T) {
		ms := new(consumertest.MetricsSink)
		mr, err := factory.CreateMetrics(context.Background(), cs, cfg, ms)
		assert.NoError(t, err)
		err = mr.Start(context.Background(), nil)
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, mr.Shutdown(context.Background()))
		}()
		err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		time.Sleep(1 * time.Second)
		require.Empty(t, ms.AllMetrics())
	})

	t.Run("trace receiver", func(t *testing.T) {
		ts := new(consumertest.TracesSink)
		tr, err := factory.CreateTraces(context.Background(), cs, cfg, ts)
		assert.NoError(t, err)
		err = tr.Start(context.Background(), nil)
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, tr.Shutdown(context.Background()))
		}()
		err = os.WriteFile(filepath.Join(tempFolder, "traces.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		time.Sleep(1 * time.Second)
		require.Empty(t, ts.AllTraces())
	})

	t.Run("log receiver", func(t *testing.T) {
		ls := new(consumertest.LogsSink)
		lr, err := factory.CreateLogs(context.Background(), cs, cfg, ls)
		assert.NoError(t, err)
		err = lr.Start(context.Background(), nil)
		assert.NoError(t, err)
		defer func() {
			assert.NoError(t, lr.Shutdown(context.Background()))
		}()
		err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		time.Sleep(1 * time.Second)
		require.Empty(t, ls.AllLogs())
	})
}

func TestFileRecordAttributes(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	cfg.IncludeFileRecordOffset = true
	cfg.IncludeFileRecordNumber = true

	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, receiver.Shutdown(context.Background()))
	}()

	ld := testdata.GenerateLogs(1)
	marshaler := &plog.JSONMarshaler{}
	b, err := marshaler.MarshalLogs(ld)
	require.NoError(t, err)
	b = append(b, '\n')

	err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), b, 0o600)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllLogs(), 1)

	// Verify file record attributes are present when enabled
	logs := sink.AllLogs()[0]
	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	offsetVal, exists := logRecord.Attributes().Get("log.file.record_offset")
	assert.True(t, exists, "log.file.record_offset should exist when enabled")
	assert.Equal(t, int64(0), offsetVal.Int(), "First record should start at offset 0")

	recordNumVal, exists := logRecord.Attributes().Get("log.file.record_number")
	assert.True(t, exists, "log.file.record_number should exist when enabled")
	assert.Equal(t, int64(1), recordNumVal.Int(), "First record should have record number 1")
}
