// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
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
	receiver, err := factory.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(t.Context(), componenttest.NewNopHost())
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
	assert.Equal(t, pd, sink.AllProfiles()[0])
	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestFileTracesReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.TracesSink)
	receiver, err := factory.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	td := testdata.GenerateTraces(1)
	marshaler := &ptrace.JSONMarshaler{}
	b, err := marshaler.MarshalTraces(td)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "traces.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	// include_file_name is true by default
	td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("log.file.name", "traces.json")

	require.Len(t, sink.AllTraces(), 1)
	assert.Equal(t, td, sink.AllTraces()[0])
	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestFileMetricsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)

	md := testdata.GenerateMetrics(1)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	// include_file_name is true by default
	md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Metadata().PutStr("log.file.name", "metrics.json")

	require.Len(t, sink.AllMetrics(), 1)
	assert.Equal(t, md, sink.AllMetrics()[0])
	err = receiver.Shutdown(t.Context())
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
	receiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(t.Context(), componenttest.NewNopHost())
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
	assert.Equal(t, md, sink.AllMetrics()[0])

	// Reset the sink and assert that the next poll replays all the existing metrics.
	sink.Reset()
	time.Sleep(cfg.PollInterval + time.Second)
	require.Len(t, sink.AllMetrics(), 1)
	assert.Equal(t, md, sink.AllMetrics()[0])

	err = receiver.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestFileLogsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.StartAt = "beginning"
	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)

	ld := testdata.GenerateLogs(1)
	marshaler := &plog.JSONMarshaler{}
	b, err := marshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), b, 0o600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	// include_file_name is true by default
	ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr("log.file.name", "logs.json")

	require.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, ld, sink.AllLogs()[0])
	err = receiver.Shutdown(t.Context())
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
	mr, err := factory.CreateMetrics(t.Context(), cs, cfg, ms)
	assert.NoError(t, err)
	err = mr.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	ts := new(consumertest.TracesSink)
	tr, err := factory.CreateTraces(t.Context(), cs, cfg, ts)
	assert.NoError(t, err)
	err = tr.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	ls := new(consumertest.LogsSink)
	lr, err := factory.CreateLogs(t.Context(), cs, cfg, ls)
	assert.NoError(t, err)
	err = lr.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	ps := new(consumertest.ProfilesSink)
	pr, err := factory.(xreceiver.Factory).CreateProfiles(t.Context(), cs, cfg, ps)
	assert.NoError(t, err)
	err = pr.Start(t.Context(), componenttest.NewNopHost())
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
	assert.Equal(t, md, ms.AllMetrics()[0])
	require.Len(t, ts.AllTraces(), 1)
	assert.Equal(t, td, ts.AllTraces()[0])
	require.Len(t, ls.AllLogs(), 1)
	assert.Equal(t, ld, ls.AllLogs()[0])
	require.Len(t, ps.AllProfiles(), 1)
	assert.Equal(t, pd, ps.AllProfiles()[0])
	err = mr.Shutdown(t.Context())
	assert.NoError(t, err)
	err = tr.Shutdown(t.Context())
	assert.NoError(t, err)
	err = lr.Shutdown(t.Context())
	assert.NoError(t, err)
	err = pr.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestEmptyLine(t *testing.T) {
	t.Run("metrics receiver", func(t *testing.T) {
		tempFolder := t.TempDir()
		cfg := createDefaultConfig().(*Config)
		cfg.Include = []string{filepath.Join(tempFolder, "*")}
		cfg.StartAt = "beginning"
		cs := receivertest.NewNopSettings(metadata.Type)
		ms := new(consumertest.MetricsSink)
		mr, err := NewFactory().CreateMetrics(t.Context(), cs, cfg, ms)
		assert.NoError(t, err)
		err = mr.Start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)
		t.Cleanup(func() {
			assert.NoError(t, mr.Shutdown(t.Context()))
		})
		err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			require.Empty(tt, ms.AllMetrics())
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("trace receiver", func(t *testing.T) {
		tempFolder := t.TempDir()
		cfg := createDefaultConfig().(*Config)
		cfg.Include = []string{filepath.Join(tempFolder, "*")}
		cfg.StartAt = "beginning"
		cs := receivertest.NewNopSettings(metadata.Type)
		ts := new(consumertest.TracesSink)
		tr, err := NewFactory().CreateTraces(t.Context(), cs, cfg, ts)
		assert.NoError(t, err)
		err = tr.Start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)
		t.Cleanup(func() {
			assert.NoError(t, tr.Shutdown(t.Context()))
		})
		err = os.WriteFile(filepath.Join(tempFolder, "traces.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			require.Empty(tt, ts.AllTraces())
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("log receiver", func(t *testing.T) {
		tempFolder := t.TempDir()
		cfg := createDefaultConfig().(*Config)
		cfg.Include = []string{filepath.Join(tempFolder, "*")}
		cfg.StartAt = "beginning"
		cs := receivertest.NewNopSettings(metadata.Type)
		ls := new(consumertest.LogsSink)
		lr, err := NewFactory().CreateLogs(t.Context(), cs, cfg, ls)
		assert.NoError(t, err)
		err = lr.Start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)
		t.Cleanup(func() {
			assert.NoError(t, lr.Shutdown(t.Context()))
		})
		err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), []byte{'\n', '\n'}, 0o600)
		assert.NoError(t, err)
		require.EventuallyWithT(t, func(tt *assert.CollectT) {
			require.Empty(tt, ls.AllLogs())
		}, time.Second, 10*time.Millisecond)
	})
}
